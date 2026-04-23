import Foundation

/// Sliding-window async helpers for running work in parallel with a
/// hard cap on in-flight tasks. Wraps the TaskGroup "prime the pool,
/// enqueue one new task per completion" pattern behind a call-site
/// one-liner so individual features don't reinvent it each time they
/// need parallelism (company logo refresh, CV chunk embedding, batch
/// APNs sends, etc.).
///
/// Why not just spawn N tasks at once? Three reasons:
///
///   1. URLSession's per-host connection pool is bounded. Firing 200
///      downloads at once just queues them serially in the socket
///      layer while burning memory holding them all as tasks.
///   2. Low-end devices choke on hundreds of simultaneous Task
///      allocations; throughput peaks well before then.
///   3. Remote endpoints (MinIO, embeddings providers) rate-limit.
///
/// A small `maxConcurrent` (8 is a good default for HTTP work) keeps
/// the network busy without any of that. Use a higher value for CPU
/// work that blocks briefly; use a lower one when the downstream is
/// sensitive.
extension Sequence where Element: Sendable {
    /// Runs `transform` for each element with at most `maxConcurrent`
    /// tasks in flight, collecting the results.
    ///
    /// Results are returned in **completion order, not input order** —
    /// fast tasks land first. Use this when you only need aggregates
    /// (counts, sums, first failure) or when order doesn't matter.
    func concurrentMap<T: Sendable>(
        maxConcurrent: Int,
        transform: @Sendable @escaping (Element) async -> T
    ) async -> [T] {
        precondition(maxConcurrent > 0, "maxConcurrent must be positive")
        return await withTaskGroup(of: T.self) { group in
            var iterator = self.makeIterator()

            // Prime the pool — spin up the first `maxConcurrent` tasks.
            for _ in 0..<maxConcurrent {
                guard let item = iterator.next() else { break }
                group.addTask { await transform(item) }
            }

            // Slide the window: each completion triggers the next
            // enqueue until the input is exhausted, then drain the
            // still-running tasks.
            var results: [T] = []
            while let result = await group.next() {
                results.append(result)
                if let item = iterator.next() {
                    group.addTask { await transform(item) }
                }
            }
            return results
        }
    }

    /// Runs `body` for each element in parallel with at most
    /// `maxConcurrent` tasks in flight. Discards results — use when
    /// the work is side-effect only (downloads, file writes, pushes).
    func concurrentForEach(
        maxConcurrent: Int,
        body: @Sendable @escaping (Element) async -> Void
    ) async {
        _ = await self.concurrentMap(maxConcurrent: maxConcurrent, transform: body)
    }
}
