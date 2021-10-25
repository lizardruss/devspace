package dependency

import (
	"golang.org/x/sync/errgroup"
)

type scheduler struct {
	concurrency int
	next        func() (func() error, error)
}

func NewScheduler(concurrency int, next func() (func() error, error)) *scheduler {
	return &scheduler{
		concurrency: concurrency,
		next:        next,
	}
}

func (s *scheduler) Run() error {
	errors := new(errgroup.Group)

	for idx := 0; idx < s.concurrency; idx++ {
		errors.Go(func() error {
			for {
				// Lookup the next piece of work
				work, err := s.next()

				// Exit if the work lookup failed
				if err != nil {
					return err
				}

				// Exit if there's no work left
				if work == nil {
					break
				}

				// Execute the work
				err = work()

				// Exit if the work failed
				if err != nil {
					return err
				}
			}
			return nil
		})
	}

	err := errors.Wait()
	if err != nil {
		return err
	}

	return nil
}
