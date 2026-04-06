import Foundation

enum StringKey {
    // Tabs
    case tabJobs, tabAlerts, tabSettings, tabProfile

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
    case alertsEmpty, alertsEmptyDescription, alertsBannerTagline

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

    // No notifications permission
    case noNotifPermissionTitle, noNotifPermissionBody, noNotifPermissionButton
}

enum Strings {
    static func get(_ key: StringKey, language: AppLanguage) -> String {
        table[language]?[key] ?? table[.spanish]![key]!
    }

    // swiftlint:disable line_length
    private static let table: [AppLanguage: [StringKey: String]] = [
        .english: [
            .tabJobs: "Jobs",
            .tabAlerts: "Alerts",
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

            .alertsEmpty: "No Alerts Yet",
            .alertsEmptyDescription: "Received notifications will appear here.",
            .alertsBannerTagline: "Push Notifications",

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
            .jobsBannerTagline: "Job Intelligence",
            .jobsBannerMatchCount: "matches today",

            .profileAnonymous: "Anonymous",
            .profileCVTitle: "Curriculum Vitae",
            .profileCVNone: "No CV uploaded",
            .profileCVHint: "Upload your CV to start matching",
            .profileCVUpload: "Upload CV",
            .profileCVUploadPending: "Coming soon via falcon-api",
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

            .noNotifPermissionTitle: "Notifications disabled",
            .noNotifPermissionBody: "Enable notifications so Falcon can alert you when a matching project is found.",
            .noNotifPermissionButton: "Enable Notifications",
        ],

        .german: [
            .tabJobs: "Jobs",
            .tabAlerts: "Alerts",
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

            .configAPIURL: "API URL",
            .configUserID: "Benutzer-ID",
            .configRegister: "Mit API registrieren",
            .configRegistering: "Wird registriert…",
            .configRegistered: "Registriert ✓",

            .tokenNone: "Noch kein Token",
            .tokenCopy: "Kopieren",

            .lastNotifNone: "Noch keine empfangen",

            .langAppLabel: "App-Sprache",

            .alertsEmpty: "Keine Benachrichtigungen",
            .alertsEmptyDescription: "Empfangene Benachrichtigungen erscheinen hier.",
            .alertsBannerTagline: "Push-Benachrichtigungen",

            .jobsTitle: "Jobs",

            .aboutCraftedIn: "Mit ♥ in Hamburg gebaut",
            .aboutCreatedBy: "Erstellt von",
            .aboutVersion: "Version",
            .aboutBuild: "Build",
            .aboutGdprNote: "CV-Embeddings werden lokal verarbeitet (Ollama — keine Daten verlassen unsere Server). Die Matching-Analyse erfolgt durch Mistral AI, einen externen Dienst mit Sitz in Frankreich (EU). Mit der Nutzung von Falcon stimmen Sie der Verarbeitung Ihrer Lebenslaufdaten durch Mistral AI zu.",
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
            .tabStats: "Stats",
            .jobsBannerTagline: "Job-Intelligenz",
            .jobsBannerMatchCount: "Matches heute",

            .profileAnonymous: "Anonym",
            .profileCVTitle: "Lebenslauf",
            .profileCVNone: "Kein Lebenslauf hochgeladen",
            .profileCVHint: "Lade deinen Lebenslauf hoch, um Matches zu starten",
            .profileCVUpload: "Lebenslauf hochladen",
            .profileCVUploadPending: "Demnächst via falcon-api",
            .profileSkillsTitle: "Deine Skills",
            .profileSkillsNone: "Skills erscheinen, sobald dein Lebenslauf verarbeitet wurde",

            .legalLastUpdated: "Zuletzt aktualisiert",

            .statsMatchesTotal: "Matches",
            .statsAvgScore: "Ø Score",
            .statsAlertsTotal: "Alerts",
            .statsMatchHistory: "Match-Verlauf",
            .statsTopSkills: "Gefragte Skills",
            .statsTopSkillsEmpty: "Daten erscheinen nach dem ersten Match",

            .noCVWarningTitle: "Noch keine Job-Alerts",
            .noCVWarningBody: "Lade deinen Lebenslauf hoch, damit Falcon dich mit passenden Projekten matcht und Alerts sendet.",

            .noNotifPermissionTitle: "Benachrichtigungen deaktiviert",
            .noNotifPermissionBody: "Aktiviere Benachrichtigungen, damit Falcon dich bei einem passenden Projekt informieren kann.",
            .noNotifPermissionButton: "Benachrichtigungen aktivieren",
        ],

        .spanish: [
            .tabJobs: "Empleos",
            .tabAlerts: "Alertas",
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

            .configAPIURL: "API URL",
            .configUserID: "ID de usuario",
            .configRegister: "Registrar con API",
            .configRegistering: "Registrando…",
            .configRegistered: "Registrado ✓",

            .tokenNone: "Sin token aún",
            .tokenCopy: "Copiar",

            .lastNotifNone: "Ninguna recibida aún",

            .langAppLabel: "Idioma de la app",

            .alertsEmpty: "Sin alertas aún",
            .alertsEmptyDescription: "Las notificaciones recibidas aparecerán aquí.",
            .alertsBannerTagline: "Notificaciones push",

            .jobsTitle: "Empleos",

            .aboutCraftedIn: "Hecho con ♥ en Hamburgo",
            .aboutCreatedBy: "Creado por",
            .aboutVersion: "Versión",
            .aboutBuild: "Build",
            .aboutGdprNote: "Los embeddings del CV se generan localmente (Ollama — sin datos fuera de nuestros servidores). El análisis de matching lo realiza Mistral AI, un servicio externo con sede en Francia (UE). Al usar Falcon aceptas que tus datos del CV sean procesados por Mistral AI.",
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

            .splashSubtitle: "Inteligencia de empleo",

            .tabProfile: "Perfil",
            .tabStats: "Stats",
            .jobsBannerTagline: "Inteligencia de empleo",
            .jobsBannerMatchCount: "matches hoy",

            .profileAnonymous: "Anónimo",
            .profileCVTitle: "Currículum Vitae",
            .profileCVNone: "Sin CV cargado",
            .profileCVHint: "Sube tu CV para comenzar a hacer matches",
            .profileCVUpload: "Subir CV",
            .profileCVUploadPending: "Próximamente via falcon-api",
            .profileSkillsTitle: "Tus habilidades",
            .profileSkillsNone: "Las habilidades aparecerán cuando tu CV sea procesado",

            .legalLastUpdated: "Última actualización",

            .statsMatchesTotal: "Matches",
            .statsAvgScore: "Score prom.",
            .statsAlertsTotal: "Alertas",
            .statsMatchHistory: "Historial",
            .statsTopSkills: "Skills más demandados",
            .statsTopSkillsEmpty: "Los datos aparecerán tras tu primer match",

            .noCVWarningTitle: "Sin alertas de empleo",
            .noCVWarningBody: "Sube tu CV para que Falcon te conecte con proyectos relevantes y te envíe alertas.",

            .noNotifPermissionTitle: "Notificaciones desactivadas",
            .noNotifPermissionBody: "Activa las notificaciones para que Falcon te avise cuando encuentre un proyecto compatible.",
            .noNotifPermissionButton: "Activar notificaciones",
        ],
    ]
    // swiftlint:enable line_length
}
