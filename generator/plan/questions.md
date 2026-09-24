# Unresolved Questions

These choices are not required to understand the target architecture, but they should be decided before implementation.

No unresolved questions at this point. A per-job PostgreSQL advisory lock on the same connection as the graph checkpointer protects both checkpoints and job-table writes across generator instances. Unfinished jobs are resumable when the lock becomes available; no separate lease is planned.
