package dependency

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/loft-sh/devspace/pkg/devspace/config"
	"github.com/loft-sh/devspace/pkg/devspace/config/constants"
	"github.com/loft-sh/devspace/pkg/devspace/config/generated"
	fakegeneratedloader "github.com/loft-sh/devspace/pkg/devspace/config/generated/testing"
	"github.com/loft-sh/devspace/pkg/devspace/config/loader"
	"github.com/loft-sh/devspace/pkg/devspace/config/versions/latest"
	fakedeploy "github.com/loft-sh/devspace/pkg/devspace/deploy/testing"
	"github.com/loft-sh/devspace/pkg/util/fsutil"
	"github.com/loft-sh/devspace/pkg/util/hash"
	"github.com/loft-sh/devspace/pkg/util/log"

	"gotest.tools/assert"
)

type fakeResolver struct {
	dependencyTree *graph
}

var replaceWithHash = "replaceThisWithHash"

func (r *fakeResolver) Resolve(update bool) (*graph, error) {
	r.dependencyTree.preOrderSearch(r.dependencyTree.Root, func(n *node) (bool, error) {
		dep := n.Dependency
		directoryHash, _ := hash.DirectoryExcludes(dep.localPath, []string{".git", ".devspace"}, true)
		for _, profile := range dep.dependencyCache.Profiles {
			for key, val := range profile.Dependencies {
				if val == replaceWithHash {
					profile.Dependencies[key] = directoryHash
				}
			}
		}

		dep.deployController = &fakedeploy.FakeController{}
		dep.generatedSaver = &fakegeneratedloader.Loader{}
		return false, nil
	})
	return r.dependencyTree, nil
}

type updateAllTestCase struct {
	name             string
	files            map[string]string
	dependencyTasks  []*latest.DependencyConfig
	activeConfig     *generated.CacheConfig
	allowCyclicParam bool
	expectedErr      string
}

func TestUpdateAll(t *testing.T) {
	testCases := []updateAllTestCase{
		{
			name: "No Dependencies to update",
		},
		{
			name: "Update one dependency",
			files: map[string]string{
				"devspace.yaml":         "version: v1beta3",
				"someDir/devspace.yaml": "version: v1beta3",
			},
			dependencyTasks: []*latest.DependencyConfig{
				{
					Source: &latest.SourceConfig{
						Path: "someDir",
					},
				},
			},
			activeConfig: &generated.CacheConfig{
				Images: map[string]*generated.ImageCache{
					"default": {
						Tag: "1.15", // This will be appended to nginx during deploy
					},
				},
				Dependencies: map[string]string{},
			},
			allowCyclicParam: true,
		},
	}

	dir, err := ioutil.TempDir("", "testFolder")
	if err != nil {
		t.Fatalf("Error creating temporary directory: %v", err)
	}

	wdBackup, err := os.Getwd()
	if err != nil {
		t.Fatalf("Error getting current working directory: %v", err)
	}
	err = os.Chdir(dir)
	if err != nil {
		t.Fatalf("Error changing working directory: %v", err)
	}

	// Delete temp folder
	defer func() {
		err = os.Chdir(wdBackup)
		if err != nil {
			t.Fatalf("Error changing dir back: %v", err)
		}
		err = os.RemoveAll(dir)
		if err != nil {
			t.Fatalf("Error removing dir: %v", err)
		}
	}()

	for _, testCase := range testCases {
		for path, content := range testCase.files {
			err = fsutil.WriteToFile([]byte(content), path)
			assert.NilError(t, err, "Error writing file in testCase %s", testCase.name)
		}

		testConfig := &latest.Config{
			Dependencies: testCase.dependencyTasks,
			Profiles: []*latest.ProfileConfig{
				{
					Name: "default",
				},
			},
		}
		generatedConfig := &generated.Config{
			ActiveProfile: "default",
			Profiles: map[string]*generated.CacheConfig{
				"default": testCase.activeConfig,
			},
		}

		manager := NewManager(config.NewConfig(nil, testConfig, generatedConfig, nil, constants.DefaultConfigPath), nil, &loader.ConfigOptions{}, log.Discard)
		err = manager.UpdateAll()
		if testCase.expectedErr == "" {
			assert.NilError(t, err, "Error updating all in testCase %s", testCase.name)
		} else {
			assert.Error(t, err, testCase.expectedErr, "Wrong or no error from UpdateALl in testCase %s", testCase.name)
		}

		for path := range testCase.files {
			err = os.Remove(path)
			assert.NilError(t, err, "Error removing file in testCase %s", testCase.name)
		}
	}
}

type buildAllTestCase struct {
	name string

	files           map[string]string
	dependencyTasks []*latest.DependencyConfig
	dependencyTree  *graph
	options         BuildOptions

	expectedErr string
}

func TestBuildAll(t *testing.T) {
	dir, err := ioutil.TempDir("", "testFolder")
	if err != nil {
		t.Fatalf("Error creating temporary directory: %v", err)
	}

	wdBackup, err := os.Getwd()
	if err != nil {
		t.Fatalf("Error getting current working directory: %v", err)
	}
	err = os.Chdir(dir)
	if err != nil {
		t.Fatalf("Error changing working directory: %v", err)
	}
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Delete temp folder
	defer func() {
		err = os.Chdir(wdBackup)
		if err != nil {
			t.Fatalf("Error changing dir back: %v", err)
		}
		err = os.RemoveAll(dir)
		if err != nil {
			t.Fatalf("Error removing dir: %v", err)
		}
	}()

	testCases := []buildAllTestCase{
		{
			name: "No Dependencies to build",
		},
		{
			name:  "Build one dependency",
			files: map[string]string{},
			dependencyTasks: []*latest.DependencyConfig{
				{},
			},
			dependencyTree: newGraph(
				newNode("./", &Dependency{
					localPath:        "./",
					dependencyConfig: &latest.DependencyConfig{},
					dependencyCache: &generated.Config{
						ActiveProfile: "",
						Profiles: map[string]*generated.CacheConfig{
							"": {
								Dependencies: map[string]string{
									"": replaceWithHash,
								},
							},
						},
					},
				}),
			),
		},
	}

	for _, testCase := range testCases {
		for path, content := range testCase.files {
			err = fsutil.WriteToFile([]byte(content), path)
			assert.NilError(t, err, "Error writing file in testCase %s", testCase.name)
		}

		manager := &manager{
			config: config.Ensure(config.NewConfig(nil, &latest.Config{
				Dependencies: testCase.dependencyTasks,
			}, nil, nil, "")),
			log: log.Discard,
			resolver: &fakeResolver{
				dependencyTree: testCase.dependencyTree,
			},
		}

		_, err = manager.BuildAll(testCase.options)
		if testCase.expectedErr == "" {
			assert.NilError(t, err, "Error deploying all in testCase %s", testCase.name)
		} else {
			assert.Error(t, err, testCase.expectedErr, "Wrong or no error from DeployALl in testCase %s", testCase.name)
		}

		for path := range testCase.files {
			err = os.Remove(path)
			assert.NilError(t, err, "Error removing file in testCase %s", testCase.name)
		}
	}
}

type deployAllTestCase struct {
	name string

	files           map[string]string
	dependencyTasks []*latest.DependencyConfig
	dependencyTree  *graph
	options         DeployOptions

	expectedErr string
}

func TestDeployAll(t *testing.T) {
	dir, err := ioutil.TempDir("", "testFolder")
	if err != nil {
		t.Fatalf("Error creating temporary directory: %v", err)
	}

	wdBackup, err := os.Getwd()
	if err != nil {
		t.Fatalf("Error getting current working directory: %v", err)
	}
	err = os.Chdir(dir)
	if err != nil {
		t.Fatalf("Error changing working directory: %v", err)
	}
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Delete temp folder
	defer func() {
		err = os.Chdir(wdBackup)
		if err != nil {
			t.Fatalf("Error changing dir back: %v", err)
		}
		err = os.RemoveAll(dir)
		if err != nil {
			t.Fatalf("Error removing dir: %v", err)
		}
	}()

	testCases := []deployAllTestCase{
		{
			name: "No Dependencies to deploy",
		},
		{
			name:  "Deploy one dependency",
			files: map[string]string{},
			dependencyTasks: []*latest.DependencyConfig{
				{},
			},
			dependencyTree: newGraph(
				newNode("./", &Dependency{
					localPath:        "./",
					dependencyConfig: &latest.DependencyConfig{},
					dependencyCache: &generated.Config{
						ActiveProfile: "",
						Profiles: map[string]*generated.CacheConfig{
							"": {
								Dependencies: map[string]string{
									"": replaceWithHash,
								},
							},
						},
					},
				}),
			),
		},
	}

	for _, testCase := range testCases {
		for path, content := range testCase.files {
			err = fsutil.WriteToFile([]byte(content), path)
			assert.NilError(t, err, "Error writing file in testCase %s", testCase.name)
		}

		manager := &manager{
			config: config.Ensure(config.NewConfig(nil, &latest.Config{
				Dependencies: testCase.dependencyTasks,
			}, nil, nil, "")),
			log: log.Discard,
			resolver: &fakeResolver{
				dependencyTree: testCase.dependencyTree,
			},
		}

		_, err = manager.DeployAll(testCase.options)
		if testCase.expectedErr == "" {
			assert.NilError(t, err, "Error deploying all in testCase %s", testCase.name)
		} else {
			assert.Error(t, err, testCase.expectedErr, "Wrong or no error from DeployALl in testCase %s", testCase.name)
		}

		for path := range testCase.files {
			err = os.Remove(path)
			assert.NilError(t, err, "Error removing file in testCase %s", testCase.name)
		}
	}
}

type purgeAllTestCase struct {
	name string

	files           map[string]string
	dependencyTasks []*latest.DependencyConfig
	dependencyTree  *graph
	verboseParam    bool

	expectedErr string
}

func TestPurgeAll(t *testing.T) {
	dir, err := ioutil.TempDir("", "testFolder")
	if err != nil {
		t.Fatalf("Error creating temporary directory: %v", err)
	}
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}

	wdBackup, err := os.Getwd()
	if err != nil {
		t.Fatalf("Error getting current working directory: %v", err)
	}
	err = os.Chdir(dir)
	if err != nil {
		t.Fatalf("Error changing working directory: %v", err)
	}

	// Delete temp folder
	defer func() {
		err = os.Chdir(wdBackup)
		if err != nil {
			t.Fatalf("Error changing dir back: %v", err)
		}
		err = os.RemoveAll(dir)
		if err != nil {
			t.Fatalf("Error removing dir: %v", err)
		}
	}()

	testCases := []purgeAllTestCase{
		{
			name: "No Dependencies to purge",
		},
		{
			name: "Purge one dependency",
			files: map[string]string{
				"devspace.yaml":         "",
				"someDir/devspace.yaml": "",
			},
			dependencyTasks: []*latest.DependencyConfig{
				{},
			},
			dependencyTree: newGraph(
				newNode("./", &Dependency{
					localPath:        "./",
					dependencyConfig: &latest.DependencyConfig{},
					dependencyCache: &generated.Config{
						ActiveProfile: "",
						Profiles: map[string]*generated.CacheConfig{
							"": {
								Dependencies: map[string]string{
									"": replaceWithHash,
								},
							},
						},
					},
				}),
			),
		},
	}

	for _, testCase := range testCases {
		for path, content := range testCase.files {
			err = fsutil.WriteToFile([]byte(content), path)
			assert.NilError(t, err, "Error writing file in testCase %s", testCase.name)
		}

		manager := &manager{
			config: config.Ensure(config.NewConfig(nil, &latest.Config{
				Dependencies: testCase.dependencyTasks,
			}, nil, nil, "")),
			log: log.Discard,
			resolver: &fakeResolver{
				dependencyTree: testCase.dependencyTree,
			},
		}

		_, err = manager.PurgeAll(PurgeOptions{Verbose: testCase.verboseParam})
		if testCase.expectedErr == "" {
			assert.NilError(t, err, "Error purging all in testCase %s", testCase.name)
		} else {
			assert.Error(t, err, testCase.expectedErr, "Wrong or no error from PurgeALl in testCase %s", testCase.name)
		}

		for path := range testCase.files {
			err = os.Remove(path)
			assert.NilError(t, err, "Error removing file in testCase %s", testCase.name)
		}
	}
}

// type buildTestCase struct {
// 	name string

// 	files             map[string]string
// 	dependencyTree    *graph
// 	skipPush          bool
// 	forceDependencies bool
// 	forceBuild        bool

// 	expectedErr string
// }

// func TestBuild(t *testing.T) {
// 	dir, err := ioutil.TempDir("", "testFolder")
// 	if err != nil {
// 		t.Fatalf("Error creating temporary directory: %v", err)
// 	}
// 	dir, err = filepath.EvalSymlinks(dir)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	wdBackup, err := os.Getwd()
// 	if err != nil {
// 		t.Fatalf("Error getting current working directory: %v", err)
// 	}
// 	err = os.Chdir(dir)
// 	if err != nil {
// 		t.Fatalf("Error changing working directory: %v", err)
// 	}

// 	// Delete temp folder
// 	defer func() {
// 		err = os.Chdir(wdBackup)
// 		if err != nil {
// 			t.Fatalf("Error changing dir back: %v", err)
// 		}
// 		err = os.RemoveAll(dir)
// 		if err != nil {
// 			t.Fatalf("Error removing dir: %v", err)
// 		}
// 	}()

// 	testCases := []buildTestCase{
// 		{
// 			name: "Skipped",
// 			dependency: &Dependency{
// 				localPath: "./",
// 				dependencyCache: &generated.Config{
// 					ActiveProfile: "",
// 					Profiles: map[string]*generated.CacheConfig{
// 						"": {
// 							Dependencies: map[string]string{
// 								"": replaceWithHash,
// 							},
// 						},
// 					},
// 				},
// 			},
// 		},
// 		{
// 			name: "Build dependency",
// 			dependency: &Dependency{
// 				localPath:        "./",
// 				dependencyConfig: &latest.DependencyConfig{},
// 				dependencyCache: &generated.Config{
// 					ActiveProfile: "",
// 					Profiles: map[string]*generated.CacheConfig{
// 						"": {
// 							Dependencies: map[string]string{
// 								"": "",
// 							},
// 						},
// 					},
// 				},
// 				buildController: &fakebuild.FakeController{
// 					BuiltImages: map[string]string{
// 						"": "",
// 					},
// 				},
// 			},
// 			forceDependencies: true,
// 		},
// 	}

// 	for _, testCase := range testCases {
// 		for path, content := range testCase.files {
// 			err = fsutil.WriteToFile([]byte(content), path)
// 			assert.NilError(t, err, "Error writing file in testCase %s", testCase.name)
// 		}

// 		dependencies, _ := (&fakeResolver{
// 			dependencyTree: testCase.dependencyTree,
// 		}).Resolve(false)
// 		dependency := dependencies.Root.childs[0].Dependency

// 		err = dependency.Build(testCase.forceDependencies, &build.Options{
// 			SkipPush:     testCase.skipPush,
// 			ForceRebuild: testCase.forceBuild,
// 		}, log.Discard)

// 		if testCase.expectedErr == "" {
// 			assert.NilError(t, err, "Error purging all in testCase %s", testCase.name)
// 		} else {
// 			assert.Error(t, err, testCase.expectedErr, "Wrong or no error from PurgeALl in testCase %s", testCase.name)
// 		}

// 		err = os.Chdir(dir)
// 		assert.NilError(t, err, "Error changing workDir back in testCase %s", testCase.name)
// 		for path := range testCase.files {
// 			err = os.Remove(path)
// 			assert.NilError(t, err, "Error removing file in testCase %s", testCase.name)
// 		}
// 	}
// }

// type deployTestCase struct {
// 	name string

// 	files             map[string]string
// 	dependencyTree    *graph
// 	skipPush          bool
// 	forceDependencies bool
// 	skipBuild         bool
// 	forceBuild        bool
// 	skipDeploy        bool
// 	forceDeploy       bool

// 	expectedErr string
// }

// func TestDeploy(t *testing.T) {
// 	dir, err := ioutil.TempDir("", "testFolder")
// 	if err != nil {
// 		t.Fatalf("Error creating temporary directory: %v", err)
// 	}
// 	dir, err = filepath.EvalSymlinks(dir)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	wdBackup, err := os.Getwd()
// 	if err != nil {
// 		t.Fatalf("Error getting current working directory: %v", err)
// 	}
// 	err = os.Chdir(dir)
// 	if err != nil {
// 		t.Fatalf("Error changing working directory: %v", err)
// 	}

// 	// Delete temp folder
// 	defer func() {
// 		err = os.Chdir(wdBackup)
// 		if err != nil {
// 			t.Fatalf("Error changing dir back: %v", err)
// 		}
// 		err = os.RemoveAll(dir)
// 		if err != nil {
// 			t.Fatalf("Error removing dir: %v", err)
// 		}
// 	}()

// 	testCases := []deployTestCase{
// 		{
// 			name: "Skipped",
// 			dependency: &Dependency{
// 				localPath: "./",
// 				dependencyCache: &generated.Config{
// 					ActiveProfile: "",
// 					Profiles: map[string]*generated.CacheConfig{
// 						"": {
// 							Dependencies: map[string]string{
// 								"": replaceWithHash,
// 							},
// 						},
// 					},
// 				},
// 			},
// 		},
// 		{
// 			name: "Deploy dependency",
// 			dependency: &Dependency{
// 				localPath:        "./",
// 				dependencyConfig: &latest.DependencyConfig{},
// 				dependencyCache: &generated.Config{
// 					ActiveProfile: "",
// 					Profiles: map[string]*generated.CacheConfig{
// 						"": {
// 							Dependencies: map[string]string{
// 								"": "",
// 							},
// 						},
// 					},
// 				},
// 				kubeClient:     &fakekube.Client{},
// 				registryClient: &fakeregistry.Client{},
// 				buildController: &fakebuild.FakeController{
// 					BuiltImages: map[string]string{
// 						"": "",
// 					},
// 				},
// 			},
// 			forceDependencies: true,
// 		},
// 	}

// 	for _, testCase := range testCases {
// 		for path, content := range testCase.files {
// 			err = fsutil.WriteToFile([]byte(content), path)
// 			assert.NilError(t, err, "Error writing file in testCase %s", testCase.name)
// 		}

// 		dependencies, _ := (&fakeResolver{
// 			resolvedDependencies: []*Dependency{
// 				testCase.dependency,
// 			},
// 		}).Resolve(false)
// 		dependency := dependencies[0]
// 		if dependency.localConfig == nil {
// 			dependency.localConfig = config.NewConfig(nil, &latest.Config{}, nil, nil, constants.DefaultConfigPath)
// 		}

// 		err = dependency.Deploy(testCase.forceDependencies, testCase.skipBuild, testCase.skipDeploy, testCase.forceDeploy, &build.Options{
// 			SkipPush:     testCase.skipPush,
// 			ForceRebuild: testCase.forceBuild,
// 		}, log.Discard)

// 		if testCase.expectedErr == "" {
// 			assert.NilError(t, err, "Error purging all in testCase %s", testCase.name)
// 		} else {
// 			assert.Error(t, err, testCase.expectedErr, "Wrong or no error from PurgeALl in testCase %s", testCase.name)
// 		}

// 		err = os.Chdir(dir)
// 		assert.NilError(t, err, "Error changing workDir back in testCase %s", testCase.name)
// 		for path := range testCase.files {
// 			err = os.Remove(path)
// 			assert.NilError(t, err, "Error removing file in testCase %s", testCase.name)
// 		}
// 	}
// }
