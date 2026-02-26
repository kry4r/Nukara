import SwiftUI

extension Color {
    // MARK: - Core palette (Avocado Milkshake theme)
    static let nkBackground = Color(hex: "#F5F8EF")
    static let nkPrimaryText = Color(hex: "#2C2C2C")
    static let nkAccent = Color(hex: "#7BA05B")
    static let nkSecondaryAccent = Color(hex: "#C4A882")

    // MARK: - Extended tokens
    static let nkAccentDark = Color(hex: "#5C7A3F")
    static let nkAccentSecondary = Color(hex: "#FFF8E7")
    static let nkTextSecondary = Color(hex: "#6B6B6B")
    static let nkTextMuted = Color(hex: "#9A9A9A")
    static let nkTextOnAccent = Color.white
    static let nkCardBackground = Color.white
    static let nkInputBackground = Color(hex: "#F7F9F3")
    static let nkWarmBackground = Color(hex: "#F0F5E8")
    static let nkBorderDefault = Color(hex: "#E5E8DE")
    static let nkBubbleUser = Color(hex: "#7BA05B")
    static let nkBubbleBot = Color(hex: "#FFFDF7")

    init(hex: String) {
        let hex = hex.trimmingCharacters(in: CharacterSet.alphanumerics.inverted)
        var int: UInt64 = 0
        Scanner(string: hex).scanHexInt64(&int)

        let a, r, g, b: UInt64
        switch hex.count {
        case 8:
            (a, r, g, b) = (int >> 24, int >> 16 & 0xFF, int >> 8 & 0xFF, int & 0xFF)
        default:
            (a, r, g, b) = (255, int >> 16, int >> 8 & 0xFF, int & 0xFF)
        }

        self.init(
            .sRGB,
            red: Double(r) / 255,
            green: Double(g) / 255,
            blue: Double(b) / 255,
            opacity: Double(a) / 255
        )
    }
}
