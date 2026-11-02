## Queue

Work too slow for a request runs in the background: a service puts a job on a Redis stream with
`pkg/queue`, and a worker, in that service or another, runs it. A job can run more than once, so
running it twice must do no harm. The `queue` skill says how.
