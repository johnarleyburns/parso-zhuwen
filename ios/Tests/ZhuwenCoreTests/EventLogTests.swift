import XCTest
@testable import ZhuwenCore

final class EventLogTests: XCTestCase {
    private let t0 = Date(timeIntervalSince1970: 1_700_000_000)

    func testAppendPreservesOrder() {
        let log = EventLog()
        log.append(.exposure(1, at: t0))
        log.append(.lookup(2, at: t0.addingTimeInterval(1)))
        log.append(.markKnown(3, at: t0.addingTimeInterval(2)))
        XCTAssertEqual(log.count, 3)
        XCTAssertEqual(log.events.map { $0.kind }, [.exposure, .lookup, .markKnown])
        XCTAssertEqual(log.events.map { $0.wordID }, [1, 2, 3])
    }

    func testEventsForWordFilters() {
        let log = EventLog([
            .exposure(1, at: t0),
            .lookup(1, at: t0.addingTimeInterval(1)),
            .exposure(2, at: t0.addingTimeInterval(2)),
        ])
        XCTAssertEqual(log.events(for: 1).count, 2)
        XCTAssertEqual(log.events(for: 2).count, 1)
        XCTAssertTrue(log.events(for: 99).isEmpty)
    }

    func testEventCodableRoundTrips() throws {
        let events = [
            Event.exposure(1, storyID: "s1", at: t0),
            Event.reviewGrade(2, grade: 3, at: t0),
            Event.comprehension(3, correct: false, storyID: "s1", at: t0),
        ]
        let data = try JSONEncoder().encode(events)
        let back = try JSONDecoder().decode([Event].self, from: data)
        XCTAssertEqual(back, events)
    }
}
