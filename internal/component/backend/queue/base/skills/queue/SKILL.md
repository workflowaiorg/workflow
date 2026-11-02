---
name: queue
description: "How work runs in the background: a job type, putting a job on the queue through a gateway, a worker that runs it through a queue handler, retries, and testing with miniredis."
---

# Skill: Queue

Work too slow to do while someone waits runs in the background. A service puts a job on a Redis
stream, and a worker, in that service or another, runs it. `pkg/queue` does the stream work; a
service never calls Redis itself.

## When this skill applies

- Adding a job: work a request starts but should not wait for
- Putting a job on the queue, or running one in a worker
- Changing what happens when a job fails
- Testing a job's worker or the code that enqueues it

## How a job runs

`queue.Work` reads one stream until the service stops. For each job it calls the function it was
given, with a `ctx` that ends after `queue.RunFor` (50 seconds), then:

| The function | The job |
|---|---|
| returns `nil` | is deleted from the stream |
| returns an error or panics, or the service stops first | runs again after `queue.RetryAfter` (a minute), in this worker or another |
| has failed `queue.MaxRuns` (5) times | is dropped, with an error logged |

A job that cannot be read as the job type is dropped at once, with an error logged.

- **A job may run more than once**: again after any failure, and after a worker stopped half-way.
  Running it again must do no harm. Check whether the work is already done before doing it, or
  make the write one that can repeat, such as an upsert.
- **A job finishes within `RunFor`**, and stops when its `ctx` ends. One that goes on past
  `RetryAfter` is handed to a second worker while the first still runs it. Work that takes longer
  is split into jobs that each enqueue the next.
- A job's error is logged as a warning by `Work`; the function does not log it again.

## A job type: `pkg/jobs/<verb>_<noun>.go`

The job is a struct with `json` tags, and its stream is a constant beside it. It is in the root
`pkg/` because the service that enqueues it and the one that runs it both import it.

```go
package jobs

// SendReceiptStream is the stream of SendReceipt jobs.
const SendReceiptStream = "orders:send-receipt"

// SendReceipt mails the receipt of a paid order.
type SendReceipt struct {
    OrderID int64 `json:"order_id"`
}
```

- The name says the action, `SendReceipt`, not the thing, `Receipt`.
- The stream is `<service>:<action>`, after the service that owns the work.
- The job carries IDs, not copies of rows: by the time it runs, the row may have changed.
- A field added later is read as its zero value from jobs already waiting; a field renamed or
  removed is lost from them.

## Enqueuing: a gateway

A controller that starts a job depends on an interface of its own, as for any gateway. The gateway
is `internal/gateway/<domain>/queue/`, and returns an `APIError`, never a Redis error:

```go
func (g *Gateway) SendReceipt(ctx context.Context, orderID int64) error {
    if err := g.queue.Enqueue(ctx, jobs.SendReceiptStream, jobs.SendReceipt{OrderID: orderID}); err != nil {
        return errors.New(errors.CodeInternal, "Could not send the receipt", err)
    }
    return nil
}
```

The route builds it from the `App`: `receiptqueue.New(application.Queue)`.

## Running: a queue handler

A worker is a transport like HTTP: `internal/handler/<domain>/queue/` has one method per job, which
calls the controller and returns its error.

```go
func (h *Handler) SendReceipt(ctx context.Context, job jobs.SendReceipt) error {
    return h.ctrl.SendReceipt(ctx, job.OrderID)
}
```

The route starts it, with the `ctx` `main` passes, which ends when the service is told to stop:

```go
application.Workers.Go(func() {
    queue.Work(ctx, application.Queue, jobs.SendReceiptStream, handler.SendReceipt)
})
```

`main` waits for every worker to stop before it closes the queue. A job cut short by the stop runs
again later.

## A new service with the queue

Copied from `api`, it already reads `REDIS_ADDR` and has the `Queue` on its `App`. In
`docker-compose.yaml`, give it `depends_on: redis: condition: service_healthy`, as `api` has.

## Testing

- `pkg/queue` is tested against [miniredis](https://github.com/alicebob/miniredis), an in-process
  Redis: `queue.New(miniredis.RunT(t).Addr())`.
- The queue handler and the gateway are tested like any other: the handler with a stand-in
  controller, the gateway against miniredis, reading back what it put on the stream.
- The controller's work is tested in the controller, with no queue at all.

## Definition of done

- [ ] The job is a struct with `json` tags in `pkg/jobs/`, named for its action, with its stream
- [ ] Running a job twice does no harm, and it finishes within `RunFor`
- [ ] Enqueuing goes through a gateway that returns an `APIError`
- [ ] The worker is a queue handler that calls the controller, started with `application.Workers.Go`
- [ ] The handler, the gateway and the controller each have tests
