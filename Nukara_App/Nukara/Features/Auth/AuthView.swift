import SwiftUI

private enum AuthMode: String, CaseIterable, Identifiable {
    case login = "登录"
    case register = "注册"

    var id: String { rawValue }

    var codePurpose: AuthCodePurpose {
        switch self {
        case .login: return .login
        case .register: return .register
        }
    }
}

@MainActor
struct AuthView: View {
    @EnvironmentObject private var sessionStore: SessionStore

    @State private var mode: AuthMode = .login
    @State private var email: String = ""
    @State private var emailCode: String = ""
    @State private var nickname: String = ""

    var body: some View {
        ScrollView {
            VStack(spacing: 14) {
                Picker("模式", selection: $mode) {
                    ForEach(AuthMode.allCases) { item in
                        Text(item.rawValue).tag(item)
                    }
                }
                .pickerStyle(.segmented)

                fieldRow(icon: "envelope.fill", title: "邮箱") {
                    TextField("请输入邮箱", text: $email)
                        .keyboardType(.emailAddress)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                        .frame(maxWidth: .infinity, minHeight: 44)
                }

                if mode == .register {
                    fieldRow(icon: "person.fill", title: "昵称") {
                        TextField("设置昵称", text: $nickname)
                            .frame(maxWidth: .infinity, minHeight: 44)
                    }
                }

                fieldRow(icon: "number.square.fill", title: "验证码") {
                    HStack(spacing: 8) {
                        TextField("请输入验证码", text: $emailCode)
                            .keyboardType(.numberPad)
                            .frame(maxWidth: .infinity, minHeight: 44)
                            .padding(.horizontal, 12)
                            .background(
                                RoundedRectangle(cornerRadius: 12, style: .continuous)
                                    .fill(Color.nkBackground.opacity(0.7))
                            )

                        Button {
                            Task {
                                await sessionStore.requestEmailCode(email: email, purpose: mode.codePurpose)
                            }
                        } label: {
                            if sessionStore.isRequestingCode {
                                ProgressView()
                                    .padding(.horizontal, 8)
                            } else {
                                Text("获取")
                                    .font(.caption.weight(.semibold))
                                    .foregroundStyle(Color.nkPrimaryText)
                            }
                        }
                        .frame(minWidth: 74, minHeight: 44)
                        .background(
                            RoundedRectangle(cornerRadius: 12, style: .continuous)
                                .fill(Color.nkAccent.opacity(0.5))
                        )
                    }
                }

                if let tip = sessionStore.tipsMessage {
                    messageRow(text: tip, color: Color.nkPrimaryText.opacity(0.75))
                }

                if let error = sessionStore.errorMessage {
                    messageRow(text: error, color: .red)
                }

                Button {
                    Task {
                        if mode == .login {
                            await sessionStore.login(email: email, emailCode: emailCode)
                        } else {
                            await sessionStore.register(email: email, emailCode: emailCode, nickname: nickname)
                        }
                    }
                } label: {
                    if sessionStore.isLoading {
                        ProgressView()
                            .frame(maxWidth: .infinity)
                            .padding(.vertical, 14)
                    } else {
                        Text(mode == .login ? "立即登录" : "创建账号")
                            .font(.headline)
                            .foregroundStyle(Color.nkPrimaryText)
                            .frame(maxWidth: .infinity)
                            .padding(.vertical, 14)
                    }
                }
                .background(
                    RoundedRectangle(cornerRadius: 14, style: .continuous)
                        .fill(Color.nkAccent)
                )
                .disabled(sessionStore.isLoading)
            }
            .padding(16)
            .background(
                RoundedRectangle(cornerRadius: 20, style: .continuous)
                    .fill(Color.white.opacity(0.93))
            )
            .padding(.horizontal, 18)
            .padding(.vertical, 24)
        }
        .background(backgroundView.ignoresSafeArea())
        .navigationTitle("登录")
        .navigationBarTitleDisplayMode(.inline)
        .onChange(of: mode) {
            sessionStore.clearMessages()
        }
    }

    private func fieldRow<Content: View>(icon: String, title: String, @ViewBuilder content: () -> Content) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(spacing: 6) {
                Image(systemName: icon)
                    .foregroundStyle(Color.nkPrimaryText.opacity(0.72))
                Text(title)
                    .font(.caption)
                    .foregroundStyle(Color.nkPrimaryText.opacity(0.72))
            }

            content()
                .padding(.horizontal, 12)
                .padding(.vertical, 8)
                .background(
                    RoundedRectangle(cornerRadius: 12, style: .continuous)
                        .fill(Color.nkBackground.opacity(0.58))
                )
        }
    }

    private func messageRow(text: String, color: Color) -> some View {
        Text(text)
            .font(.caption)
            .foregroundStyle(color)
            .frame(maxWidth: .infinity, alignment: .leading)
    }

    private var backgroundView: some View {
        LinearGradient(
            colors: [Color.nkBackground, Color.nkSecondaryAccent.opacity(0.18), Color.nkAccent.opacity(0.15)],
            startPoint: .top,
            endPoint: .bottom
        )
    }
}
