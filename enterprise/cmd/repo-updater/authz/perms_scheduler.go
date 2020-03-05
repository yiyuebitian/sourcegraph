package authz

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"gopkg.in/inconshreveable/log15.v2"
)

// TODO: docstring
type PermsScheduler struct {
	// The time duration of how often a schedule happens.
	internal time.Duration
}

// TODO: docstring
func NewPermsScheduler(internal time.Duration) *PermsScheduler {
	return &PermsScheduler{
		internal: internal,
	}
}

// TODO: docstring
func (s *PermsScheduler) scanUsersWithNoPerms(ctx context.Context) ([]int32, error) {
	// TODO
	return nil, nil
}

// TODO: docstring
func (s *PermsScheduler) scanReposWithNoPerms(ctx context.Context) ([]api.RepoID, error) {
	// TODO
	return nil, nil
}

// TODO: docstring
func (s *PermsScheduler) scanUsersWithOldestPerms(ctx context.Context) ([]int32, error) {
	// TODO
	return nil, nil
}

// TODO: docstring
func (s *PermsScheduler) scanReposWithOldestPerms(ctx context.Context) ([]api.RepoID, error) {
	// TODO
	return nil, nil
}

// TODO: docstring
// We scan and schedule four sets of results in a particular order because:
//   1. Users with no permissions stored can't do anything meaningful (e.g. not able to search).
//   2. Private repositories with no permissions stored can't be viewed anyone except site admins.
//   3. Rolling updating user permissions over time. This is done before the 4. is because GitLab
//      OAuth authz provider is not capable of doing repository-centric permissions syncing. Doing
//      this step first and combine with threshold will help reduce noop requests.
//   4. Rolling updating repository permissions over time.
func (s *PermsScheduler) schedule(ctx context.Context, syncer *PermsSyncer) error {
	// TODO(jchen): Predict a threshold based on total repos and users that make sense to finish syncing
	// before next scans (use the formula defined in RFC 113), so we don't waste database bandwidth.

	userIDs, err := s.scanUsersWithNoPerms(ctx)
	if err != nil {
		return errors.Wrap(err, "scan users with no permissions")
	}
	err = syncer.ScheduleUsers(ctx, PriorityHigh, userIDs...)
	if err != nil {
		return errors.Wrap(err, "schedule requests for users with no permissions")
	}

	repoIDs, err := s.scanReposWithNoPerms(ctx)
	if err != nil {
		return errors.Wrap(err, "scan repositories with no permissions")
	}
	err = syncer.ScheduleRepos(ctx, PriorityHigh, repoIDs...)
	if err != nil {
		return errors.Wrap(err, "schedule requests for repositories with no permissions")
	}

	userIDs, err = s.scanUsersWithOldestPerms(ctx)
	if err != nil {
		return errors.Wrap(err, "scan users with oldest permissions")
	}
	err = syncer.ScheduleUsers(ctx, PriorityLow, userIDs...)
	if err != nil {
		return errors.Wrap(err, "schedule requests for users with oldest permissions")
	}

	repoIDs, err = s.scanReposWithOldestPerms(ctx)
	if err != nil {
		return errors.Wrap(err, "scan repositories with oldest permissions")
	}
	err = syncer.ScheduleRepos(ctx, PriorityLow, repoIDs...)
	if err != nil {
		return errors.Wrap(err, "schedule requests for repositories with oldest permissions")
	}

	return nil
}

// TODO: docstring
func StartPermsSyncing(ctx context.Context, scheduler *PermsScheduler, syncer *PermsSyncer) {
	go syncer.Run(ctx)

	log15.Debug("started perms scheduler")
	defer log15.Info("stopped perms scheduler")

	ticker := time.NewTicker(scheduler.internal)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return
		}

		err := scheduler.schedule(ctx, syncer)
		if err != nil {
			log15.Error("Failed to schedule permissions syncing", "err", err)
			continue
		}
	}
}
