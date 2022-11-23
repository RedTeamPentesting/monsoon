The program creates a processing pipeline consisting of the following elements:

 * Sources: emit a sequence of values in a deterministic way that are to be
   inserted into requests. Implemented are a range source (which can be
   configured with a format string) and a file source which emits each line
   of a file.

 * Multiplexer: If several sources are to be used, the multiplexer combines
   them and computes the cross product. It reads values from all sources and
   sends tuples of strings to the next stage.

 * ValueFilter: filters the sequence of tuples emitted by the multiplexer. Can
   be used to skip the first n tuples (`--skip`) and limit the number of tuples
   processed (`--limit`).

 * Limiter: optional, limits the throughput of items to the runners, can be
   used to only process a number of items per second.

 * Runners: take the tuples, builds HTTP requests by replacing strings and
   sends them to the server. Emit a sequence of responses. Multiple Runners are
   working in parallel, so the sequence of responses is not deterministic any
   more and highly depends on the server.

 * ResponseFilter: decides for each HTTP response if it should be rejected
   according to the current configuration.

 * Extracter: runs external commands to extract data. Since this is rather
   expensive, we only do it for non-hidden responses.

 * Reporter: takes the HTTP responses from the Runners, runs the filters on
   each one and displays the responses not rejected by the filter to the user,
   in addition to statistics and runtime information.

This is a rough diagram of how it all fits together:

```
+--------+                                                         +--------+
| Source +-+                                                    +->| Runner |-+
+--------+ |                                                    |  +--------+ |
           |  +-------------+   +-------------+   +---------+   |             |   +----------------+   +------------+   +----------+
  ...      +->| Multiplexer +-->| ValueFilter +-->| Limiter +---+->  ...      +-->| ResponseFilter +-->|  Extracter +-->| Reporter |
           |  +-------------+   +-------------+   +---------+   |             |   +----------------+   +------------+   +----------+
+--------+ |                                                    |  +--------+ |
| Source +-+                                                    +->| Runner |-+
+--------+                                                         +--------+
```
