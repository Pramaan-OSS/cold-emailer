import SwiftUI

/// Mailbox authorization is owned by Pramaan Connections in the web workspace.
struct MailboxConnectFlow: View {
    var onClose: () -> Void = {}
    var onConnected: ((EmailAccount) -> Void)? = nil
    var body: some View {
        VStack(spacing: 20) {
            Image(systemName: "envelope.badge.shield.half.filled").font(.largeTitle)
            Text("Connect a mailbox").font(.title2.bold())
            Text("Open Connections in your web workspace to connect or create a mailbox. Direct mailbox authorization is unavailable in this app.")
                .multilineTextAlignment(.center)
            Button("Close", action: onClose).buttonStyle(.borderedProminent)
        }.padding(24)
    }
}
