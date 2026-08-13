# Request scheduling

Both cluster-scoped and namespaced `Request` resources support optional resource-specific drift observation schedules:

```yaml
spec:
  forProvider:
    pollInterval: 24h
    pollJitter: 1h
```

When `pollInterval` is omitted, the provider's global `--poll` interval remains in effect. `pollJitter` requires an explicit interval and adds a deterministic non-negative offset in `[0, pollJitter]`. The offset remains stable for a resource and schedule across provider restarts.

The provider persists:

- `status.lastRequestTime`: when it last sent external HTTP.
- `status.nextPollTime`: when periodic observation next becomes due.
- `status.rateLimitUntil`: the active HTTP 429 deferral deadline.
- `status.observedDesiredStateHash`: the desired state evaluated by the last external request.

A provider restart preserves a future persisted schedule. Changes to Request spec, labels, or non-ignored annotations bypass periodic waiting. For example, operators can use a changing annotation as an auditable manual prompt:

```bash
kubectl annotate request.http.crossplane.io scheduled-observation \
  example.org/reconcile-at="$(date -u +%FT%TZ)" --overwrite
```

Event-triggered reconciliation does not bypass an active HTTP 429 deferral. On 429, the provider selects a deadline in this order:

1. `Retry-After` delta-seconds or HTTP-date.
2. `X-Rate-Limit-Reset` Unix timestamp.
3. Bounded exponential fallback backoff from one minute to one hour.

The provider retains the response code, body, and headers for diagnosis and automatically retries after `status.rateLimitUntil` without requiring resource deletion or recreation.
