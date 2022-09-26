package registry

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/docker/cli/cli/streams"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/loft-sh/devspace/pkg/devspace/config/versions/latest"
	corev1 "k8s.io/api/core/v1"
)

func HasPushPermission(image *latest.Image) bool {
	ref, err := name.ParseReference(image.Image)
	if err != nil {
		panic(err)
	}

	pushErr := remote.CheckPushPermission(ref, authn.DefaultKeychain, http.DefaultTransport)
	return pushErr == nil
}

func IsLocalRegistryDisabled(config *latest.Config) bool {
	return config.LocalRegistry != nil && config.LocalRegistry.Disable
}

func GetServicePort(service *corev1.Service) *corev1.ServicePort {
	for _, port := range service.Spec.Ports {
		if port.Name == "registry" {
			return &port
		}
	}
	return nil
}

func IsImageAvailableRemotely(ctx context.Context, imageName string) (bool, error) {
	ref, err := name.ParseReference(imageName)
	if err != nil {
		return false, err
	}

	image, err := remote.Image(
		ref,
		remote.WithContext(ctx),
		remote.WithTransport(remote.DefaultTransport),
	)
	if err != nil {
		transportError, ok := err.(*transport.Error)
		if ok && transportError.StatusCode == http.StatusNotFound {
			return false, nil
		}
		return false, err
	}

	return image != nil, nil
}

func CopyImageToRemote(ctx context.Context, output io.Writer, imageName string) error {
	ref, err := name.ParseReference(imageName)
	if err != nil {
		return err
	}

	image, err := daemon.Image(ref, daemon.WithContext(ctx))
	if err != nil {
		return err
	}

	progressChan := make(chan v1.Update, 200)
	errChan := make(chan error, 1)
	go func() {
		errChan <- remote.Write(
			ref,
			image,
			remote.WithContext(ctx),
			remote.WithTransport(remote.DefaultTransport),
			remote.WithProgress(progressChan),
		)
	}()

	reader, writer := io.Pipe()
	defer writer.Close()

	outStream := streams.NewOut(output)
	outStream.SetIsTerminal(true)
	go func() {
		errChan <- jsonmessage.DisplayJSONMessagesStream(reader, outStream, outStream.FD(), outStream.IsTerminal(), nil)
	}()

	enc := json.NewEncoder(writer)
	for update := range progressChan {
		if update.Error != nil {
			return err
		}

		status := "Pushing"
		if update.Complete == update.Total {
			status = "Pushed"
		}

		err := enc.Encode(&jsonmessage.JSONMessage{
			ID:     ref.Identifier(),
			Status: status,
			Progress: &jsonmessage.JSONProgress{
				Current: update.Complete,
				Total:   update.Total,
			},
		})
		if err != nil {
			return err
		}
	}

	return <-errChan
}