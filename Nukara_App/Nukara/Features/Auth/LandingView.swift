import SwiftUI

@MainActor
struct LandingView: View {
    var body: some View {
        NavigationStack {
            ZStack {
                LinearGradient(
                    colors: [Color.nkBackground, Color.nkWarmBackground, Color.nkAccent.opacity(0.15)],
                    startPoint: .top,
                    endPoint: .bottom
                )
                .ignoresSafeArea()

                VStack {
                    Spacer()

                    VStack(spacing: 10) {
                        Text("纽扣")
                            .font(.system(size: 56, weight: .bold))
                            .foregroundStyle(Color.nkPrimaryText)
                        Text("Nukara")
                            .font(.system(size: 18, weight: .medium))
                            .foregroundStyle(Color.nkTextSecondary)
                    }

                    Spacer()

                    NavigationLink {
                        AuthView()
                    } label: {
                        ZStack {
                            Circle()
                                .fill(Color.nkAccent)
                                .frame(width: 64, height: 64)
                            Image(systemName: "arrow.right")
                                .font(.system(size: 22, weight: .semibold))
                                .foregroundStyle(Color.nkTextOnAccent)
                        }
                    }
                    .padding(.bottom, 60)
                }
            }
            .navigationBarHidden(true)
        }
    }
}
