import SwiftUI
import ZhuwenCore
import ZhuwenPacks
import ZhuwenPersistence
import ZhuwenUI

/// The `@main` entry point (MC-1). Previously assembled ad hoc in Xcode outside the repo; now
/// vendored and agent-buildable via `make app`. It owns the durable SwiftData learner store and
/// wires the persistent, append-only event log (`PersistentEventLog`) into the shared `AppModel`
/// so the learner's state survives relaunch and is rebuilt by replay (I5).
@main
struct ZhuwenApp: App {
    @Environment(\.scenePhase) private var scenePhase
    @StateObject private var app: AppModel
    private let eventLog: PersistentEventLog?

    init() {
        let resetRequested = ProcessInfo.processInfo.arguments.contains("-uiTestReset")
        let skipOnboarding = ProcessInfo.processInfo.arguments.contains("-uiTestSkipOnboarding")
        let (model, log) = ZhuwenApp.bootstrap(resetStore: resetRequested, skipOnboarding: skipOnboarding)
        _app = StateObject(wrappedValue: model)
        eventLog = log
    }

    var body: some Scene {
        WindowGroup {
            RootView(model: app)
                .overlay(alignment: .bottom) { PersistenceProbe(learner: app.learner) }
        }
        .onChange(of: scenePhase) { _, phase in
            if phase != .active { eventLog?.saveCheckpoint() }
        }
    }

    // MARK: - Bootstrap

    private static func bootstrap(resetStore: Bool, skipOnboarding: Bool) -> (AppModel, PersistentEventLog?) {
        let (log, placement) = openStores(resetStore: resetStore)
        guard let packURL = Bundle.main.url(forResource: "fixture-a2-v0", withExtension: "zpack"),
              let pubURL = Bundle.main.url(forResource: "zhuwen-dev", withExtension: "pub"),
              let pub = try? Minisign.PublicKey(file: String(contentsOf: pubURL, encoding: .utf8)),
              let store = try? PackStore(url: packURL, publicKey: pub)
        else {
            fatalError("ZhuwenApp requires bundled fixture-a2-v0.zpack + zhuwen-dev.pub")
        }
        let model = AppModel(store: store, events: log?.events ?? [], eventSink: log,
                             placementStore: placement,
                             forceRoute: skipOnboarding ? .loop : nil,
                             sync: SyncModel(enabled: false, engine: NoOpSyncEngine()))
        return (model, log)
    }

    private static func openStores(resetStore: Bool) -> (PersistentEventLog?, PersistentPlacementStore?) {
        let base = URL.applicationSupportDirectory.appending(path: "Zhuwen", directoryHint: .isDirectory)
        try? FileManager.default.createDirectory(at: base, withIntermediateDirectories: true)
        let url = base.appending(path: "learner.store")
        guard let container = try? LearnerStore.container(url: url) else { return (nil, nil) }
        let log = PersistentEventLog(container: container)
        let placement = PersistentPlacementStore(container: container)
        if resetStore {
            log.replaceAll([])
            placement.clear()
        }
        return (log, placement)
    }
}

/// A nearly-invisible overlay exposing the persisted lookup/event counts to the XCUITest smoke
/// test via `accessibilityIdentifier("persistenceProbe")`.
private struct PersistenceProbe: View {
    @ObservedObject var learner: LearnerModel

    var body: some View {
        Text("lookups \(learner.lookupCount) · events \(learner.events.count)")
            .font(.caption2)
            .foregroundColor(.secondary)
            .opacity(0.04)
            .accessibilityIdentifier("persistenceProbe")
            .allowsHitTesting(false)
    }
}
