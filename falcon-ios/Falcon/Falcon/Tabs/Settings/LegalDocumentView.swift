import SwiftUI

private struct LegalSection: Decodable {
    let title: String
    let body: String
}

private struct LegalDocument: Decodable {
    let title: String
    let lastUpdated: String
    let sections: [LegalSection]
}

struct LegalDocumentView: View {
    let filename: String
    @Environment(LanguageManager.self) private var lm

    private var document: LegalDocument? {
        let lang = lm.appLanguage.rawValue
        let localized = "\(filename)_\(lang)"
        let fallback  = "\(filename)_en"
        for name in [localized, fallback] {
            if let url  = Bundle.main.url(forResource: name, withExtension: "json"),
               let data = try? Data(contentsOf: url),
               let doc  = try? JSONDecoder().decode(LegalDocument.self, from: data) {
                return doc
            }
        }
        return nil
    }

    var body: some View {
        ScrollView {
            if let doc = document {
                VStack(alignment: .leading, spacing: 24) {
                    ForEach(doc.sections, id: \.title) { section in
                        VStack(alignment: .leading, spacing: 8) {
                            Text(section.title)
                                .font(.system(size: 15, weight: .semibold))
                            Text(section.body)
                                .font(.system(size: 14))
                                .foregroundStyle(.secondary)
                                .fixedSize(horizontal: false, vertical: true)
                        }
                    }
                    Text("\(lm.t(.legalLastUpdated)): \(doc.lastUpdated)")
                        .font(.caption)
                        .foregroundStyle(.tertiary)
                        .padding(.top, 8)
                }
                .padding(.horizontal, 20)
                .padding(.top, 20)
                .padding(.bottom, 100)
            }
        }
        .navigationTitle(document?.title ?? "")
        .navigationBarTitleDisplayMode(.large)
    }
}
