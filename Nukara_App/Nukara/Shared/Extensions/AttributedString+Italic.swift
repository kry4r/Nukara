import SwiftUI

extension AttributedString {
    /// Parses `*text*` patterns and applies italic styling.
    static func chatFormatted(_ text: String) -> AttributedString {
        var result = AttributedString()
        var remaining = text[...]

        while let starIdx = remaining.firstIndex(of: "*") {
            // Append text before the *
            let before = remaining[remaining.startIndex..<starIdx]
            result.append(AttributedString(String(before)))

            let afterStar = remaining.index(after: starIdx)
            guard afterStar < remaining.endIndex else {
                result.append(AttributedString("*"))
                remaining = remaining[remaining.endIndex...]
                break
            }

            let rest = remaining[afterStar...]
            if let closeIdx = rest.firstIndex(of: "*"), closeIdx > afterStar {
                var italic = AttributedString(String(rest[afterStar..<closeIdx]))
                italic.font = .body.italic()
                result.append(italic)
                remaining = remaining[remaining.index(after: closeIdx)...]
            } else {
                result.append(AttributedString("*"))
                remaining = remaining[afterStar...]
            }
        }

        if !remaining.isEmpty {
            result.append(AttributedString(String(remaining)))
        }

        return result
    }
}
