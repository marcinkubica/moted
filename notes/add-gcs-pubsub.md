# Add gcs with pubsub support

Analyse codebase and discuss detailed plan with (user before creating the plan file)

## Requirements:

- should support multiple buckets
- should support watch patterns like gs://bucket/path/**/*.md
- should support pubsub notifications for file changes (for watch patterns)
- use default application credentials for authentication (should work with GKE workload identity without any extra configuration)

## Discuss:

- restructuring config file - suggest how to organize it better
- can we keep existing groups structure?

## Output:

After confirmation from user:
- detailed plan file in notes/add-gcs-pubsub-plan.md
