import Foundation

enum StringKey {
    // Tabs
    case tabJobs, tabMatches, tabSettings, tabProfile, tabActions
    case actionsEmptyTitle, actionsEmptyBody

    // Settings sections
    case sectionNotifications, sectionConfiguration, sectionDeviceToken
    case sectionLastNotification, sectionLanguage, sectionAbout

    // Notifications
    case notifStatusLabel, notifStatusActive, notifStatusDenied
    case notifStatusProvisional, notifStatusPending
    case notifEnableButton

    // Configuration
    case configAPIURL, configUserID
    case configRegister, configRegistering, configRegistered

    // Device token
    case tokenNone, tokenCopy

    // Last notification
    case lastNotifNone

    // Language settings
    case langAppLabel

    // Alerts view
    case matchesEmpty, matchesEmptyDescription, matchesBannerTagline, matchesBannerTotal
    case matchesInfoBadge, matchesInfoBadgeTitle, matchesNewBadge
    case matchesFilterAll, matchesFilterUnread, matchesFilterEmptyUnread
    case liveNewProjectsSingular, liveNewProjectsPlural
    case liveNewMatchesSingular,  liveNewMatchesPlural
    case liveTapToRefresh
    case matchLabelApplyImmediately, matchLabelTopCandidate
    case matchLabelAcceptable, matchLabelNotSuitable
    case matchesDetails, matchesViewJob, matchesSkillsYouHave, matchesMissingSkills
    case matchesScoreBreakdown, matchesPositivePoints, matchesNegativePoints, matchesImprovementTips
    case matchesScoreSkillsMatch, matchesScoreSeniorityFit, matchesScoreDomainExperience
    case matchesScoreCommunicationClarity, matchesScoreProjectRelevance, matchesScoreTechStackOverlap

    // Jobs view — placeholder
    case jobsTitle

    // About
    case aboutCraftedIn
    case aboutCreatedBy

    // Splash
    case splashSubtitle

    // Jobs banner
    case jobsBannerTagline, jobsBannerMatchCount

    // Profile
    case profileAnonymous
    case profileCVTitle, profileCVNone, profileCVHint, profileCVUpload, profileCVUploadPending
    case profileCVDropzone, profileCVFormats
    case profileCVEmailLabel, profileCVEmailHint, profileCVEmailPlaceholder
    case profileCVUploadStart, profileCVUploading, profileCVIndexing, profileCVNormalizing, profileCVUploadDone, profileCVUploadFailed
    case profileCVSectionExperience, profileCVSectionTechnologies, profileCVSectionOthers, profileCVReplace
    case profileWhyTitle, profileWhyBody
    case profileLoginButton, profileLoginTitle, profileLoginEmailHint, profileLoginCTA
    case alreadyHaveAccountTitle, alreadyHaveAccountBody
    case authErrorTitle, authErrorTokenUsed, authErrorTokenExpired, authErrorGeneric
    case loginSentTitle, loginSentBody, loginSentSpamHint
    case profileHowTitle
    case profileStep1Title, profileStep1Body
    case profileStep2Title, profileStep2Body
    case profileStep3Title, profileStep3Body
    case profileSkillsTitle, profileSkillsNone

    // About — legal & meta
    case aboutVersion, aboutBuild
    case aboutGdprNote, aboutMistralPrivacy
    case aboutPrivacyPolicy, aboutTermsOfUse, aboutSourceCode
    case aboutCreator, aboutCompany, aboutContact
    case aboutCreatorBio, aboutCompanyBio
    case drawerVisitWebsite, drawerSendEmail, drawerCancel

    // Legal
    case legalLastUpdated

    // Stats
    case tabStats
    case statsMatchesTotal, statsAvgScore, statsAlertsTotal
    case statsMatchHistory, statsTopSkills, statsTopSkillsEmpty

    // No-CV warning
    case noCVWarningTitle, noCVWarningBody
    case cvFailedAlertTitle, cvFailedAlertBody, cvFailedAlertButton
    case profileCVProcessingFailed
    case settingsLogout
    case processingFact1, processingFact2, processingFact3, processingFact4, processingFact5
    case reviewGood, reviewAcceptable, reviewBad, reviewLabel, reviewCount
    case cvDetailTasks, cvDetailTechnologies, cvDetailShowMore, cvDetailShowLess

    // No notifications permission
    case noNotifPermissionTitle, noNotifPermissionBody, noNotifPermissionButton

    // Live Activities disabled
    case liveActivitiesDisabledTitle, liveActivitiesDisabledBody

    // Job detail
    case detailHighlights, detailMustHave, detailShouldHave, detailNiceToHave
    case detailResponsibilities, detailContact, detailViewOriginal, detailShowMore
    case detailCallContact, detailCallCTA
    case detailEmailContact, detailEmailCTA
}

enum Strings {
    static func get(_ key: StringKey, language: AppLanguage) -> String {
        table[language]?[key] ?? table[.spanish]![key]!
    }

    // swiftlint:disable line_length
    private static let table: [AppLanguage: [StringKey: String]] = [
        .english: [
            .tabJobs: "Jobs",
            .tabMatches: "Matches",
            .tabSettings: "Settings",

            .sectionNotifications: "Notifications",
            .sectionConfiguration: "Configuration",
            .sectionDeviceToken: "Device Token",
            .sectionLastNotification: "Last Notification",
            .sectionLanguage: "Language",
            .sectionAbout: "About",

            .notifStatusLabel: "Status",
            .notifStatusActive: "Active",
            .notifStatusDenied: "Denied",
            .notifStatusProvisional: "Provisional",
            .notifStatusPending: "Pending",
            .notifEnableButton: "Enable Notifications",

            .configAPIURL: "API URL",
            .configUserID: "User ID",
            .configRegister: "Register with API",
            .configRegistering: "Registering…",
            .configRegistered: "Registered ✓",

            .tokenNone: "No token yet",
            .tokenCopy: "Copy",

            .lastNotifNone: "None received yet",

            .langAppLabel: "App Language",

            .matchesEmpty: "No Matches Yet",
            .matchesEmptyDescription: "Projects matching your profile will appear here.",
            .matchesBannerTagline: "AI Matching",
            .matchesBannerTotal: "total matches",
            .matchesInfoBadge: "Matches are projects the AI thinks fit your profile, ranked by score.",
            .matchesInfoBadgeTitle: "What are matches?",
            .matchesNewBadge: "NEW",
            .matchesFilterAll:          "All",
            .matchesFilterUnread:       "Unread",
            .matchesFilterEmptyUnread:  "No unread matches",
            .liveNewProjectsSingular: "%d new project",
            .liveNewProjectsPlural:   "%d new projects",
            .liveNewMatchesSingular:  "%d new match",
            .liveNewMatchesPlural:    "%d new matches",
            .liveTapToRefresh:        "Tap to refresh",
            .matchLabelApplyImmediately: "Apply Immediately",
            .matchLabelTopCandidate: "Top Candidate",
            .matchLabelAcceptable: "Acceptable",
            .matchLabelNotSuitable: "Not Suitable",
            .matchesDetails: "Match details",
            .matchesViewJob: "View job",
            .matchesSkillsYouHave: "Skills you have",
            .matchesMissingSkills: "Missing skills",
            .matchesScoreBreakdown: "Score breakdown",
            .matchesPositivePoints: "What fits",
            .matchesNegativePoints: "Concerns",
            .matchesImprovementTips: "How to improve",
            .matchesScoreSkillsMatch: "Skills",
            .matchesScoreSeniorityFit: "Seniority",
            .matchesScoreDomainExperience: "Domain",
            .matchesScoreCommunicationClarity: "Communication",
            .matchesScoreProjectRelevance: "Relevance",
            .matchesScoreTechStackOverlap: "Tech stack",

            .jobsTitle: "Jobs",

            .aboutCraftedIn: "Crafted with ♥ in Hamburg",
            .aboutCreatedBy: "Created by",
            .aboutVersion: "Version",
            .aboutBuild: "Build",
            .aboutGdprNote: "CV embeddings are generated on-premise (Ollama — no data leaves our servers). Job matching analysis is performed by Mistral AI, an external service based in France (EU). By using Falcon you consent to your CV data being processed by Mistral AI for matching purposes.",
            .aboutMistralPrivacy: "Mistral AI Privacy Policy",
            .aboutCreator: "Developer",
            .aboutCreatorBio: "Full Stack Developer with %d+ years of experience, specializing in building scalable real-time architectures.",
            .drawerVisitWebsite: "Visit Website",
            .drawerSendEmail: "Send Email",
            .drawerCancel: "Cancel",
            .aboutCompanyBio: "A top-notch web design and development team helping businesses craft meaningful and interactive product experiences.",
            .aboutCompany: "Development Team",
            .aboutContact: "Contact",
            .aboutPrivacyPolicy: "Privacy Policy",
            .aboutTermsOfUse: "Terms of Use",
            .aboutSourceCode: "Source Code",

            .splashSubtitle: "Job Intelligence",

            .tabProfile: "Profile",
            .tabStats: "Stats",
            .tabActions: "Actions",
            .actionsEmptyTitle: "Nothing to do",
            .actionsEmptyBody: "Your account and notifications are all set up.",
            .jobsBannerTagline: "Job Intelligence",
            .jobsBannerMatchCount: "job offers today",

            .profileAnonymous: "Anonymous",
            .profileCVTitle: "Curriculum Vitae",
            .profileCVNone: "No CV uploaded",
            .profileCVHint: "Upload your CV to start matching",
            .profileCVUpload: "Upload CV",
            .profileCVUploadPending: "Coming soon via falcon-api",
            .profileCVDropzone: "Tap to select your CV",
            .profileCVFormats: "Word Document (.docx)",
            .profileCVEmailLabel: "Email",
            .profileCVEmailHint: "We'll use this to send you match alerts",
            .profileCVEmailPlaceholder: "your@email.com",
            .profileCVUploadStart: "Upload CV",
            .profileCVUploading: "Uploading…",
            .profileCVIndexing: "Processing…",
            .profileCVNormalizing: "Analyzing…",
            .profileCVUploadDone: "CV uploaded successfully",
            .profileCVUploadFailed: "Upload failed. Try again.",
            .profileCVSectionExperience: "Experience",
            .profileCVSectionTechnologies: "Technologies",
            .profileCVSectionOthers: "Others",
            .profileCVReplace: "Replace CV",
            .profileWhyTitle: "Get matched with top projects",
            .profileWhyBody: "Upload your CV and let Falcon match you with the most relevant freelance projects. Our AI analyzes your skills and experience to surface the best opportunities.",
            .profileLoginButton: "Sign In",
            .profileLoginTitle: "Sign in",
            .profileLoginEmailHint: "We'll send a magic link to your inbox.",
            .profileLoginCTA: "Send login link",
            .alreadyHaveAccountTitle: "Already have an account?",
            .alreadyHaveAccountBody: "Sign in to sync your CV and matches across devices.",
            .authErrorTitle: "Login failed",
            .authErrorTokenUsed: "This link has already been used. Please request a new one.",
            .authErrorTokenExpired: "This link has expired. Please request a new one.",
            .authErrorGeneric: "Something went wrong. Please try again.",
            .loginSentTitle: "Check your email",
            .loginSentBody: "We sent a login link to **%@**. Tap it to log in.",
            .loginSentSpamHint: "Don't see it? Check your spam folder.",
            .profileHowTitle: "How it works",
            .profileStep1Title: "Upload your CV",
            .profileStep1Body: "Share your experience and skills as a Word document.",
            .profileStep2Title: "AI analysis",
            .profileStep2Body: "Falcon extracts your skills, experience and tech stack automatically.",
            .profileStep3Title: "Get matched",
            .profileStep3Body: "Receive personalized project recommendations based on your profile.",
            .profileSkillsTitle: "Your Skills",
            .profileSkillsNone: "Skills will appear once your CV is processed",

            .legalLastUpdated: "Last updated",

            .statsMatchesTotal: "Total Matches",
            .statsAvgScore: "Avg Score",
            .statsAlertsTotal: "Alerts",
            .statsMatchHistory: "Match History",
            .statsTopSkills: "Top Skills Demanded",
            .statsTopSkillsEmpty: "Data will appear after your first match",

            .noCVWarningTitle: "No job alerts yet",
            .noCVWarningBody: "Upload your CV so Falcon can match you with relevant projects and send you alerts.",
            .cvFailedAlertTitle: "CV processing failed",
            .cvFailedAlertBody: "Something went wrong while processing your CV. Please try uploading it again.",
            .cvFailedAlertButton: "Go to Profile",
            .profileCVProcessingFailed: "Processing failed. Please try again.",
            .settingsLogout: "Log out",
            .processingFact1: "We use Mistral AI to ensure GDPR compliance",
            .processingFact2: "Your data stays on servers located in Germany",
            .processingFact3: "We analyze public job listings to find matches",
            .processingFact4: "Your CV is processed securely and never shared",
            .processingFact5: "Matching happens automatically once your profile is ready",
            .reviewGood: "Good Recruiter",
            .reviewAcceptable: "Acceptable Recruiter",
            .reviewBad: "Bad Recruiter",
            .reviewLabel: "Recruiter",
            .reviewCount: "reviews",
            .cvDetailTasks: "Tasks",
            .cvDetailTechnologies: "Technologies",
            .cvDetailShowMore: "Show more...",
            .cvDetailShowLess: "Show less",

            .noNotifPermissionTitle: "Notifications disabled",
            .noNotifPermissionBody: "Enable notifications so Falcon can alert you when a matching project is found.",
            .noNotifPermissionButton: "Enable Notifications",

            .liveActivitiesDisabledTitle: "Live Activities disabled",
            .liveActivitiesDisabledBody: "Enable Live Activities and frequent updates in Settings to see scores on your Lock Screen and Dynamic Island.",

            .detailHighlights: "Highlights",
            .detailMustHave: "Must Have",
            .detailShouldHave: "Should Have",
            .detailNiceToHave: "Nice to Have",
            .detailResponsibilities: "Responsibilities",
            .detailContact: "Contact",
            .detailViewOriginal: "View Original Posting",
            .detailShowMore: "Show more",
            .detailCallContact: "Call",
            .detailCallCTA: "Call",
            .detailEmailContact: "Send email",
            .detailEmailCTA: "Send email",
        ],

        .german: [
            .tabJobs: "Jobs",
            .tabMatches: "Treffer",
            .tabSettings: "Einstellungen",

            .sectionNotifications: "Benachrichtigungen",
            .sectionConfiguration: "Konfiguration",
            .sectionDeviceToken: "Gerät-Token",
            .sectionLastNotification: "Letzte Benachrichtigung",
            .sectionLanguage: "Sprache",
            .sectionAbout: "Über",

            .notifStatusLabel: "Status",
            .notifStatusActive: "Aktiv",
            .notifStatusDenied: "Abgelehnt",
            .notifStatusProvisional: "Vorläufig",
            .notifStatusPending: "Ausstehend",
            .notifEnableButton: "Benachrichtigungen aktivieren",

            .configAPIURL: "API-URL",
            .configUserID: "Benutzer-ID",
            .configRegister: "Mit API registrieren",
            .configRegistering: "Wird registriert…",
            .configRegistered: "Registriert ✓",

            .tokenNone: "Noch kein Token",
            .tokenCopy: "Kopieren",

            .lastNotifNone: "Noch keine erhalten",

            .langAppLabel: "App-Sprache",

            .matchesEmpty: "Noch keine Treffer",
            .matchesEmptyDescription: "Projekte, die zu deinem Profil passen, erscheinen hier.",
            .matchesBannerTagline: "KI-gestützte Treffer",
            .matchesBannerTotal: "Treffer gesamt",
            .matchesInfoBadge: "Treffer sind Projekte, die laut KI zu deinem Profil passen, sortiert nach Punktzahl.",
            .matchesInfoBadgeTitle: "Was sind Treffer?",
            .matchesNewBadge: "NEU",
            .matchesFilterAll:          "Alle",
            .matchesFilterUnread:       "Ungelesen",
            .matchesFilterEmptyUnread:  "Keine ungelesenen Treffer",
            .liveNewProjectsSingular: "%d neues Projekt",
            .liveNewProjectsPlural:   "%d neue Projekte",
            .liveNewMatchesSingular:  "%d neuer Treffer",
            .liveNewMatchesPlural:    "%d neue Treffer",
            .liveTapToRefresh:        "Tippen zum Aktualisieren",
            .matchesDetails: "Treffer-Details",
            .matchesViewJob: "Zum Job",
            .matchesSkillsYouHave: "Vorhandene Fähigkeiten",
            .matchesMissingSkills: "Fehlende Fähigkeiten",
            .matchesScoreBreakdown: "Bewertung im Detail",
            .matchesPositivePoints: "Stärken",
            .matchesNegativePoints: "Bedenken",
            .matchesImprovementTips: "Verbesserungsvorschläge",
            .matchesScoreSkillsMatch: "Fähigkeiten",
            .matchesScoreSeniorityFit: "Seniorität",
            .matchesScoreDomainExperience: "Branche",
            .matchesScoreCommunicationClarity: "Kommunikation",
            .matchesScoreProjectRelevance: "Relevanz",
            .matchesScoreTechStackOverlap: "Tech-Stack",
            .matchLabelApplyImmediately: "Sofort bewerben",
            .matchLabelTopCandidate: "Top-Kandidat",
            .matchLabelAcceptable: "Akzeptabel",
            .matchLabelNotSuitable: "Ungeeignet",

            .jobsTitle: "Jobs",

            .aboutCraftedIn: "Entwickelt Mit ♥ in Hamburg",
            .aboutCreatedBy: "Erstellt von",
            .aboutVersion: "Version",
            .aboutBuild: "Build-Version",
            .aboutGdprNote: "CV-Embeddings werden lokal verarbeitet (Ollama — keine Daten verlassen unsere Server). Die Analyse passender Treffer erfolgt durch Mistral AI, einen externen Dienst mit Sitz in Frankreich (EU). Mit der Nutzung von Falcon stimmen Sie der Verarbeitung Ihrer Lebenslaufdaten durch Mistral AI zu.",
            .aboutMistralPrivacy: "Datenschutzerklärung von Mistral AI",
            .aboutCreator: "Entwickler",
            .aboutCreatorBio: "Full-Stack-Entwickler mit %d+ Jahren Erfahrung, spezialisiert auf skalierbare Architekturen für Echtzeitsysteme.",
            .drawerVisitWebsite: "Website besuchen",
            .drawerSendEmail: "E-Mail senden",
            .drawerCancel: "Abbrechen",
            .aboutCompanyBio: "Ein erstklassiges Web-Design- und Entwicklungsteam, das Unternehmen dabei unterstützt, bedeutungsvolle und interaktive Produkterlebnisse zu gestalten.",
            .aboutCompany: "Entwicklungsteam",
            .aboutContact: "Kontakt",
            .aboutPrivacyPolicy: "Datenschutzerklärung",
            .aboutTermsOfUse: "Nutzungsbedingungen",
            .aboutSourceCode: "Quellcode",

            .splashSubtitle: "Job-Intelligenz",

            .tabProfile: "Profil",
            .tabStats: "Statistiken",
            .tabActions: "Aktionen",
            .actionsEmptyTitle: "Nichts zu tun",
            .actionsEmptyBody: "Dein Konto und die Benachrichtigungen sind vollständig eingerichtet.",
            .jobsBannerTagline: "Intelligente Jobsuche",
            .jobsBannerMatchCount: "Stellenangebote heute",

            .profileAnonymous: "Anonym",
            .profileCVTitle: "Lebenslauf",
            .profileCVNone: "Kein Lebenslauf hochgeladen",
            .profileCVHint: "Lade deinen Lebenslauf hoch, um passende Treffer zu erhalten",
            .profileCVUpload: "Lebenslauf hochladen",
            .profileCVUploadPending: "Demnächst via falcon-api",
            .profileCVDropzone: "Lebenslauf auswählen",
            .profileCVFormats: "Word Document (.docx)",
            .profileCVEmailLabel: "E-Mail",
            .profileCVEmailHint: "Damit schicken wir dir passende Job-Alerts",
            .profileCVEmailPlaceholder: "deine@email.de",
            .profileCVUploadStart: "Lebenslauf hochladen",
            .profileCVUploading: "Wird hochgeladen…",
            .profileCVIndexing: "Wird verarbeitet…",
            .profileCVNormalizing: "Wird analysiert…",
            .profileCVUploadDone: "Lebenslauf erfolgreich hochgeladen",
            .profileCVUploadFailed: "Fehler beim Hochladen. Erneut versuchen.",
            .profileCVSectionExperience: "Berufserfahrung",
            .profileCVSectionTechnologies: "Technologien",
            .profileCVSectionOthers: "Sonstige",
            .profileCVReplace: "Lebenslauf ersetzen",
            .profileWhyTitle: "Passende Projekte finden",
            .profileWhyBody: "Lade deinen Lebenslauf hoch und lass Falcon die relevantesten Freelance-Projekte für dich finden. Unsere KI analysiert deine Fähigkeiten und Erfahrungen.",
            .profileLoginButton: "Anmelden",
            .profileLoginTitle: "Anmelden",
            .profileLoginEmailHint: "Wir schicken dir einen Anmeldelink per E-Mail.",
            .profileLoginCTA: "Anmeldelink senden",
            .alreadyHaveAccountTitle: "Schon ein Konto?",
            .alreadyHaveAccountBody: "Melde dich an, um Lebenslauf und Treffer geräteübergreifend zu synchronisieren.",
            .authErrorTitle: "Anmeldung fehlgeschlagen",
            .authErrorTokenUsed: "Dieser Link wurde bereits verwendet. Bitte fordere einen neuen an.",
            .authErrorTokenExpired: "Dieser Link ist abgelaufen. Bitte fordere einen neuen an.",
            .authErrorGeneric: "Etwas ist schiefgelaufen. Bitte versuche es erneut.",
            .loginSentTitle: "Prüfe dein E-Mail-Postfach",
            .loginSentBody: "Wir haben einen Anmeldelink an **%@** gesendet. Tippe darauf, um dich anzumelden.",
            .loginSentSpamHint: "Nicht gefunden? Überprüfe deinen Spam-Ordner.",
            .profileHowTitle: "So funktioniert es",
            .profileStep1Title: "Lebenslauf hochladen",
            .profileStep1Body: "Teile deine Erfahrung und Fähigkeiten als Word-Dokument.",
            .profileStep2Title: "KI-Analyse",
            .profileStep2Body: "Falcon extrahiert automatisch deine Fähigkeiten, Erfahrung und Technologien.",
            .profileStep3Title: "Treffer erhalten",
            .profileStep3Body: "Erhalte personalisierte Projektempfehlungen passend zu deinem Profil.",
            .profileSkillsTitle: "Deine Fähigkeiten",
            .profileSkillsNone: "Fähigkeiten erscheinen, sobald dein Lebenslauf verarbeitet wurde",

            .legalLastUpdated: "Zuletzt aktualisiert",

            .statsMatchesTotal: "Treffer",
            .statsAvgScore: "Ø Bewertung",
            .statsAlertsTotal: "Benachrichtigungen",
            .statsMatchHistory: "Trefferverlauf",
            .statsTopSkills: "Gefragte Fähigkeiten",
            .statsTopSkillsEmpty: "Daten erscheinen nach dem ersten Treffer",

            .noCVWarningTitle: "Noch keine Job-Benachrichtigungen",
            .noCVWarningBody: "Lade deinen Lebenslauf hoch, damit Falcon passende Projekte für dich findet und dir Benachrichtigungen sendet.",
            .cvFailedAlertTitle: "Lebenslauf-Verarbeitung fehlgeschlagen",
            .cvFailedAlertBody: "Beim Verarbeiten deines Lebenslaufs ist ein Fehler aufgetreten. Bitte lade ihn erneut hoch.",
            .cvFailedAlertButton: "Zum Profil",
            .profileCVProcessingFailed: "Verarbeitung fehlgeschlagen. Bitte erneut versuchen.",
            .settingsLogout: "Abmelden",
            .processingFact1: "Wir nutzen Mistral AI für DSGVO-Konformität",
            .processingFact2: "Deine Daten bleiben auf Servern in Deutschland",
            .processingFact3: "Wir analysieren öffentliche Stellenangebote, um passende Treffer zu finden",
            .processingFact4: "Dein Lebenslauf wird sicher verarbeitet und nie weitergegeben",
            .processingFact5: "Passende Treffer werden automatisch ermittelt, sobald dein Profil bereit ist",
            .reviewGood: "Guter Vermittler",
            .reviewAcceptable: "Akzeptabler Vermittler",
            .reviewBad: "Schlechter Vermittler",
            .reviewLabel: "Vermittler",
            .reviewCount: "Bewertungen",
            .cvDetailTasks: "Aufgaben",
            .cvDetailTechnologies: "Technologien",
            .cvDetailShowMore: "Mehr anzeigen...",
            .cvDetailShowLess: "Weniger anzeigen",

            .noNotifPermissionTitle: "Benachrichtigungen deaktiviert",
            .noNotifPermissionBody: "Aktiviere Benachrichtigungen, damit Falcon dich bei einem passenden Projekt informieren kann.",
            .noNotifPermissionButton: "Benachrichtigungen aktivieren",

            .liveActivitiesDisabledTitle: "Live-Aktivitäten deaktiviert",
            .liveActivitiesDisabledBody: "Aktiviere in den Einstellungen Live-Aktivitäten und häufige Updates, um Treffer auf Sperrbildschirm und Dynamic Island zu sehen.",

            .detailHighlights: "Wichtige Punkte",
            .detailMustHave: "Pflichtanforderungen",
            .detailShouldHave: "Sollte mitbringen",
            .detailNiceToHave: "Von Vorteil",
            .detailResponsibilities: "Aufgaben",
            .detailContact: "Kontakt",
            .detailViewOriginal: "Original-Ausschreibung öffnen",
            .detailShowMore: "Mehr anzeigen",
            .detailCallContact: "Anrufen",
            .detailCallCTA: "Anrufen",
            .detailEmailContact: "E-Mail senden",
            .detailEmailCTA: "E-Mail senden",
        ],

        .spanish: [
            .tabJobs: "Empleos",
            .tabMatches: "Coincidencias",
            .tabSettings: "Ajustes",

            .sectionNotifications: "Notificaciones",
            .sectionConfiguration: "Configuración",
            .sectionDeviceToken: "Token del dispositivo",
            .sectionLastNotification: "Última notificación",
            .sectionLanguage: "Idioma",
            .sectionAbout: "Acerca de",

            .notifStatusLabel: "Estado",
            .notifStatusActive: "Activo",
            .notifStatusDenied: "Denegado",
            .notifStatusProvisional: "Provisional",
            .notifStatusPending: "Pendiente",
            .notifEnableButton: "Activar notificaciones",

            .configAPIURL: "URL de la API",
            .configUserID: "ID de usuario",
            .configRegister: "Registrar con API",
            .configRegistering: "Registrando…",
            .configRegistered: "Registrado ✓",

            .tokenNone: "Sin token aún",
            .tokenCopy: "Copiar",

            .lastNotifNone: "Ninguna recibida aún",

            .langAppLabel: "Idioma de la app",

            .matchesEmpty: "Sin coincidencias aún",
            .matchesEmptyDescription: "Los proyectos que coincidan con tu perfil aparecerán aquí.",
            .matchesBannerTagline: "Coincidencias con IA",
            .matchesBannerTotal: "coincidencias totales",
            .matchesInfoBadge: "Las coincidencias son proyectos que según la IA encajan con tu perfil, ordenadas por puntuación.",
            .matchesInfoBadgeTitle: "¿Qué son las coincidencias?",
            .matchesNewBadge: "NUEVO",
            .matchesFilterAll:          "Todos",
            .matchesFilterUnread:       "No leídos",
            .matchesFilterEmptyUnread:  "No hay coincidencias sin leer",
            .liveNewProjectsSingular: "%d nuevo proyecto",
            .liveNewProjectsPlural:   "%d nuevos proyectos",
            .liveNewMatchesSingular:  "%d nueva coincidencia",
            .liveNewMatchesPlural:    "%d nuevas coincidencias",
            .liveTapToRefresh:        "Toca para actualizar",
            .matchesDetails: "Ver detalles",
            .matchesViewJob: "Ver oferta",
            .matchesSkillsYouHave: "Habilidades que tienes",
            .matchesMissingSkills: "Habilidades que faltan",
            .matchesScoreBreakdown: "Puntuación detallada",
            .matchesPositivePoints: "Lo que encaja",
            .matchesNegativePoints: "Puntos de atención",
            .matchesImprovementTips: "Cómo mejorar",
            .matchesScoreSkillsMatch: "Habilidades",
            .matchesScoreSeniorityFit: "Seniority",
            .matchesScoreDomainExperience: "Sector",
            .matchesScoreCommunicationClarity: "Comunicación",
            .matchesScoreProjectRelevance: "Relevancia",
            .matchesScoreTechStackOverlap: "Tech stack",
            .matchLabelApplyImmediately: "Aplica ya",
            .matchLabelTopCandidate: "Top candidato",
            .matchLabelAcceptable: "Aceptable",
            .matchLabelNotSuitable: "No apto",

            .jobsTitle: "Empleos",

            .aboutCraftedIn: "Hecho con ♥ en Hamburgo",
            .aboutCreatedBy: "Creado por",
            .aboutVersion: "Versión",
            .aboutBuild: "Compilación",
            .aboutGdprNote: "Los embeddings del CV se generan localmente (Ollama — sin datos fuera de nuestros servidores). El análisis de coincidencias lo realiza Mistral AI, un servicio externo con sede en Francia (UE). Al usar Falcon aceptas que tus datos del CV sean procesados por Mistral AI.",
            .aboutMistralPrivacy: "Política de privacidad de Mistral AI",
            .aboutCreator: "Desarrollador",
            .aboutCreatorBio: "Desarrollador Full Stack con %d+ años de experiencia, especializado en arquitecturas escalables para sistemas en tiempo real.",
            .drawerVisitWebsite: "Visitar sitio web",
            .drawerSendEmail: "Enviar correo",
            .drawerCancel: "Cancelar",
            .aboutCompanyBio: "Un equipo de diseño y desarrollo web de primer nivel que ayuda a las empresas a crear experiencias de producto significativas e interactivas.",
            .aboutCompany: "Equipo de desarrollo",
            .aboutContact: "Contacto",
            .aboutPrivacyPolicy: "Política de privacidad",
            .aboutTermsOfUse: "Términos de uso",
            .aboutSourceCode: "Código fuente",

            .splashSubtitle: "IA para proyectos Freelance",

            .tabProfile: "Perfil",
            .tabStats: "Estadísticas",
            .tabActions: "Acciones",
            .actionsEmptyTitle: "Nada pendiente",
            .actionsEmptyBody: "Tu cuenta y notificaciones están al día.",
            .jobsBannerTagline: "Inteligencia laboral",
            .jobsBannerMatchCount: "ofertas laborales hoy",

            .profileAnonymous: "Anónimo",
            .profileCVTitle: "Currículum Vitae",
            .profileCVNone: "Sin CV cargado",
            .profileCVHint: "Sube tu CV para empezar a recibir coincidencias",
            .profileCVUpload: "Subir CV",
            .profileCVUploadPending: "Próximamente via falcon-api",
            .profileCVDropzone: "Toca para seleccionar tu CV",
            .profileCVFormats: "Word Document (.docx)",
            .profileCVEmailLabel: "Email",
            .profileCVEmailHint: "Lo usaremos para enviarte alertas de empleo",
            .profileCVEmailPlaceholder: "tu@email.com",
            .profileCVUploadStart: "Subir CV",
            .profileCVUploading: "Subiendo…",
            .profileCVIndexing: "Procesando…",
            .profileCVNormalizing: "Analizando…",
            .profileCVUploadDone: "CV subido correctamente",
            .profileCVUploadFailed: "Error al subir. Inténtalo de nuevo.",
            .profileCVSectionExperience: "Experiencia",
            .profileCVSectionTechnologies: "Tecnologías",
            .profileCVSectionOthers: "Otros",
            .profileCVReplace: "Reemplazar CV",
            .profileWhyTitle: "Consigue proyectos a tu medida",
            .profileWhyBody: "Sube tu CV y deja que Falcon te conecte con los proyectos freelance más relevantes. Nuestra IA analiza tus habilidades y experiencia para encontrar las mejores oportunidades.",
            .profileLoginButton: "Iniciar sesión",
            .profileLoginTitle: "Iniciar sesión",
            .profileLoginEmailHint: "Te enviaremos un enlace mágico a tu correo.",
            .profileLoginCTA: "Enviar enlace de acceso",
            .alreadyHaveAccountTitle: "¿Ya tienes cuenta?",
            .alreadyHaveAccountBody: "Inicia sesión para sincronizar tu CV y coincidencias entre dispositivos.",
            .authErrorTitle: "Error al iniciar sesión",
            .authErrorTokenUsed: "Este enlace ya fue utilizado. Solicita uno nuevo.",
            .authErrorTokenExpired: "Este enlace ha expirado. Solicita uno nuevo.",
            .authErrorGeneric: "Algo salió mal. Inténtalo de nuevo.",
            .loginSentTitle: "Revisa tu correo",
            .loginSentBody: "Enviamos un enlace de acceso a **%@**. Tócalo para iniciar sesión.",
            .loginSentSpamHint: "¿No lo ves? Revisa tu carpeta de spam.",
            .profileHowTitle: "Cómo funciona",
            .profileStep1Title: "Sube tu CV",
            .profileStep1Body: "Comparte tu experiencia y habilidades en un documento Word.",
            .profileStep2Title: "Análisis con IA",
            .profileStep2Body: "Falcon extrae automáticamente tus habilidades, experiencia y stack tecnológico.",
            .profileStep3Title: "Recibe coincidencias",
            .profileStep3Body: "Recibe recomendaciones de proyectos personalizadas según tu perfil.",
            .profileSkillsTitle: "Tus habilidades",
            .profileSkillsNone: "Las habilidades aparecerán cuando tu CV sea procesado",

            .legalLastUpdated: "Última actualización",

            .statsMatchesTotal: "Coincidencias",
            .statsAvgScore: "Puntuación prom.",
            .statsAlertsTotal: "Alertas",
            .statsMatchHistory: "Historial",
            .statsTopSkills: "Skills más demandados",
            .statsTopSkillsEmpty: "Los datos aparecerán tras tu primera coincidencia",

            .noCVWarningTitle: "Sin alertas de empleo",
            .noCVWarningBody: "Sube tu CV para que Falcon te conecte con proyectos relevantes y te envíe alertas.",
            .cvFailedAlertTitle: "Error al procesar tu CV",
            .cvFailedAlertBody: "Algo salió mal al procesar tu hoja de vida. Intenta subirla de nuevo.",
            .cvFailedAlertButton: "Ir al Perfil",
            .profileCVProcessingFailed: "Error al procesar. Inténtalo de nuevo.",
            .settingsLogout: "Cerrar sesión",
            .processingFact1: "Usamos Mistral AI para cumplir con el GDPR",
            .processingFact2: "Tus datos permanecen en servidores en Alemania",
            .processingFact3: "Analizamos ofertas laborales públicas para encontrar coincidencias",
            .processingFact4: "Tu CV se procesa de forma segura y nunca se comparte",
            .processingFact5: "Las coincidencias comienzan automáticamente cuando tu perfil esté listo",
            .reviewGood: "Buen Reclutador",
            .reviewAcceptable: "Reclutador Aceptable",
            .reviewBad: "Mal Reclutador",
            .reviewLabel: "Reclutador",
            .reviewCount: "reseñas",
            .cvDetailTasks: "Tareas",
            .cvDetailTechnologies: "Tecnologías",
            .cvDetailShowMore: "Ver más...",
            .cvDetailShowLess: "Ver menos",

            .noNotifPermissionTitle: "Notificaciones desactivadas",
            .noNotifPermissionBody: "Activa las notificaciones para que Falcon te avise cuando encuentre un proyecto compatible.",
            .noNotifPermissionButton: "Activar notificaciones",

            .liveActivitiesDisabledTitle: "Live Activities desactivadas",
            .liveActivitiesDisabledBody: "Activa Live Activities y las actualizaciones frecuentes en Ajustes para ver las coincidencias en la pantalla de bloqueo y Dynamic Island.",

            .detailHighlights: "Destacados",
            .detailMustHave: "Imprescindible",
            .detailShouldHave: "Deseable",
            .detailNiceToHave: "Valorable",
            .detailResponsibilities: "Responsabilidades",
            .detailContact: "Contacto",
            .detailViewOriginal: "Ver publicación original",
            .detailShowMore: "Ver más",
            .detailCallContact: "Llamar",
            .detailCallCTA: "Llamar",
            .detailEmailContact: "Enviar email",
            .detailEmailCTA: "Enviar email",
        ],
    ]
    // swiftlint:enable line_length
}
