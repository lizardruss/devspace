package dependency

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"gotest.tools/assert"
)

func TestRun(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	work := []string{"one", "two", "three", "four", "five"}
	results := []string{}

	scheduler := NewScheduler(2, func() (func() error, error) {
		if len(work) > 0 {
			result := work[0]
			work = work[1:]
			return func() error {
				time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)
				results = append(results, result)
				return nil
			}, nil
		} else {
			return nil, nil
		}
	})

	err := scheduler.Run()
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, len(results), 5)
}

func TestRunLess(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	work := []string{"one", "two"}
	results := []string{}

	scheduler := NewScheduler(5, func() (func() error, error) {
		if len(work) > 0 {
			result := work[0]
			work = work[1:]
			return func() error {
				time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)
				results = append(results, result)
				return nil
			}, nil
		} else {
			return nil, nil
		}
	})

	err := scheduler.Run()
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, len(results), 2)
}

func TestRunWorkErrors(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	work := []string{"one", "two", "three", "four", "five"}
	expectedErr := fmt.Errorf("work error!")

	scheduler := NewScheduler(2, func() (func() error, error) {
		if len(work) > 0 {
			result := work[0]
			work = work[1:]
			return func() error {
				time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)
				if result == "three" {
					return expectedErr
				}
				fmt.Println(result)
				return nil
			}, nil
		} else {
			return nil, nil
		}
	})

	err := scheduler.Run()
	if err != expectedErr {
		t.Fatal(err)
	}
}

func TestRunLookupWorkErrors(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	work := []string{"one", "two", "three", "four", "five"}
	expectedErr := fmt.Errorf("work error!")

	scheduler := NewScheduler(2, func() (func() error, error) {
		if len(work) > 0 {
			result := work[0]
			work = work[1:]
			if result == "three" {
				return nil, expectedErr
			}
			return func() error {
				time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)
				fmt.Println(result)
				return nil
			}, nil
		} else {
			return nil, nil
		}
	})

	err := scheduler.Run()
	if err != expectedErr {
		t.Fatal(err)
	}
}
