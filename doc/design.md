The program creates a processing pipeline consisting of the following items:

 * Producer: emits a sequence of values in a deterministic way that are to be
   inserted into requests instead of the string `FUZZ`. Implemented are a range
   produces (which can be configured with a format string) and a file producer
   which emits each line of a file.

 * ValueFilter: filters the sequence of items emitted by the producer. Can be
   used to skip the first n items (`--skip`) and limit the number of items
   processed (`--limit`).

 * Limiter: optional, limits the throughput of items to the runners, can be
   used to only process a number of items per second.

 * Runners: take the items, builds HTTP requests and sends them to the server.
   Emit a sequence of responses. Multiple Runners are working in parallel, so
   the sequence of responses is not deterministic any more and highly depends
   on the server.

 * ResponseFilter: decides for each HTTP response if it should be rejected
   according to the current configuration.

 * Extracter: runs external commands to extract data. Since this is rather
   expensive, we only do it for non-hidden responses.

 * Reporter: takes the HTTP responses from the Runners, runs the filters on
   each one and displays the responses not rejected by the filter to the user,
   in addition to statistics and runtime information.

This is a rough diagram of how it all fits together:

```
                                                  +--------+
                                               +->| Runner |-+
                                               |  +--------+ |   
+----------+   +-------------+   +---------+   |             |   +----------------+   +------------+   +----------+
| Producer +-->| ValueFilter +-->| Limiter +---+->  ...      +-->| ResponseFilter +-->|  Extracter +-->| Reporter |
+----------+   +-------------+   +---------+   |             |   +----------------+   +------------+   +----------+
                                               |  +--------+ |
                                               +->| Runner |-+
                                                  +--------+
```
