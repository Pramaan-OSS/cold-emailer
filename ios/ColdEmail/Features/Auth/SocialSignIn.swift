import SwiftUI
struct SocialSignInRow: View {
 let providers: AuthProvidersInfo?
 @Binding var busy: Bool
 let onError: (String)->Void
 var onTwoFA: (String)->Void={_ in}
 var body: some View { Text("Use password sign-in here. Social sign-in is available on your workspace's web page.").font(.footnote).foregroundStyle(.secondary) }
}
