package fallback

// Free pool support is split by responsibility:
//   - free_provider_registry.go: provider metadata and naming helpers
//   - free_provider_sync.go: channel/deployment sync and stale cleanup
//   - free_provider_fetch.go: upstream model and credit fetchers
//   - free_provider_scheduler.go: background sync lifecycle
