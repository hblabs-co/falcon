import Foundation

struct MatchData {
    let score: Double
    let label: String
    let summary: String
    let matchedSkills: [String]
    let missingSkills: [String]
    let skillsMatch: Double
    let seniorityFit: Double
    let domainExperience: Double
    let communicationClarity: Double
    let projectRelevance: Double
    let techStackOverlap: Double

    init(from userInfo: [AnyHashable: Any]) {
        score             = userInfo["score"] as? Double ?? 0
        label             = userInfo["label"] as? String ?? ""
        summary           = userInfo["summary"] as? String ?? ""
        matchedSkills     = userInfo["matched_skills"] as? [String] ?? []
        missingSkills     = userInfo["missing_skills"] as? [String] ?? []

        let s             = userInfo["scores"] as? [String: Double] ?? [:]
        skillsMatch          = s["skills_match"]          ?? 0
        seniorityFit         = s["seniority_fit"]         ?? 0
        domainExperience     = s["domain_experience"]     ?? 0
        communicationClarity = s["communication_clarity"] ?? 0
        projectRelevance     = s["project_relevance"]     ?? 0
        techStackOverlap     = s["tech_stack_overlap"]    ?? 0
    }

    static let preview = MatchData(from: [
        "score": 7.8,
        "label": "top_candidate",
        "summary": "Score 7.8 · React/TypeScript stark, fehlendes AWS und Docker.",
        "matched_skills": ["React", "TypeScript", "Node.js"],
        "missing_skills": ["AWS", "Docker"],
        "scores": [
            "skills_match": 8.0,
            "seniority_fit": 8.2,
            "domain_experience": 6.5,
            "communication_clarity": 9.0,
            "project_relevance": 7.0,
            "tech_stack_overlap": 7.5,
        ]
    ])
}
