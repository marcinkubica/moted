package server

import (
	"context"
	"fmt"
	"log/slog"

	"cloud.google.com/go/pubsub/v2"
	"github.com/bmatcuk/doublestar/v4"
	"github.com/k1LoW/donegroup"
)

// StartPubSubSubscribers starts a goroutine for each bucket subscription.
// Call after SetupGCS and after all patterns have been added.
func (s *State) StartPubSubSubscribers(ctx context.Context) error {
	if s.gcs == nil {
		return nil
	}

	client, err := pubsub.NewClient(ctx, s.gcs.project)
	if err != nil {
		return fmt.Errorf("failed to create Pub/Sub client: %w", err)
	}

	// Close the Pub/Sub client when the server shuts down.
	donegroup.Go(ctx, func() error {
		<-ctx.Done()
		if err := client.Close(); err != nil {
			slog.Warn("failed to close Pub/Sub client", "error", err)
		}
		return nil
	})

	for bucketName, bs := range s.gcs.buckets {
		if bs.Subscription == "" {
			continue
		}
		sub := client.Subscriber(bs.Subscription)
		bucket := bucketName // capture for goroutine

		donegroup.Go(ctx, func() error {
			slog.Info("starting Pub/Sub subscriber",
				"bucket", bucket, "subscription", bs.Subscription)
			return sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
				s.handlePubSubMessage(bucket, msg)
				msg.Ack()
			})
		})
	}
	return nil
}

// handlePubSubMessage processes a GCS notification delivered via Pub/Sub.
//
// GCS notification message attributes:
//   - bucketId: bucket name
//   - objectId: object path (e.g., "reports/2026/file.md")
//   - eventType: "OBJECT_FINALIZE" (create/update) or "OBJECT_DELETE"
func (s *State) handlePubSubMessage(bucket string, msg *pubsub.Message) {
	s.handleGCSNotification(bucket, msg.Attributes)
}

// handleGCSNotification processes GCS notification attributes.
// Separated from handlePubSubMessage for testability.
func (s *State) handleGCSNotification(bucket string, attrs map[string]string) {
	objectID := attrs["objectId"]
	eventType := attrs["eventType"]
	if objectID == "" || eventType == "" {
		slog.Warn("ignoring Pub/Sub message with missing attributes",
			"bucket", bucket, "attributes", attrs)
		return
	}

	uri := fmt.Sprintf("gs://%s/%s", bucket, objectID)

	// Match against all GCS watch patterns.
	s.mu.RLock()
	var matchedGroups []string
	for _, gp := range s.patterns {
		if !IsGCSPath(gp.Pattern) {
			continue
		}
		ok, _ := doublestar.Match(gp.PatternSlash, uri)
		if ok {
			matchedGroups = append(matchedGroups, gp.Group)
		}
	}
	s.mu.RUnlock()

	if len(matchedGroups) == 0 {
		return
	}

	slog.Info("Pub/Sub event matched", "uri", uri, "event", eventType, "groups", matchedGroups)

	switch eventType {
	case "OBJECT_FINALIZE":
		s.gcs.InvalidateCache(uri)

		existing := s.FindFileByPath(uri)
		if existing == nil {
			// New file — add to each matching group.
			for _, group := range matchedGroups {
				if _, err := s.addGCSFile(uri, group); err != nil {
					slog.Warn("failed to add GCS file from Pub/Sub", "uri", uri, "group", group, "error", err)
				}
			}
		} else {
			s.scheduleFileChanged(uri)
		}

	case "OBJECT_DELETE":
		s.RemoveFileByPath(uri)

	case "OBJECT_ARCHIVE":
		// On versioned buckets, overwrite emits OBJECT_ARCHIVE for the old version
		// with overwrittenByGeneration set. OBJECT_FINALIZE handles the new version,
		// so we ignore these.
		// Deletion on versioned buckets emits OBJECT_ARCHIVE without
		// overwrittenByGeneration — treat as delete.
		if attrs["overwrittenByGeneration"] != "" {
			return
		}
		s.RemoveFileByPath(uri)
	}
}
