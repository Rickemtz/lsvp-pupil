package ui

// Todos los textos visibles de la interfaz.
const (
	strAppTitle    = "Linuxpupil"
	strAppSubtitle = "Aprende la terminal GNU/Linux · basado en el curso del LSVP-UAMI"

	strMenuLessons     = "Lecciones"
	strMenuTimeAttack  = "Contrarreloj"
	strMenuQuickQuiz   = "Quiz rápido"
	strMenuFinalExam   = "Examen final"
	strMenuLeaderboard = "Ranking"
	strMenuAbout       = "Acerca de"
	strMenuQuit        = "Salir"

	strMenuHelp    = "↑/↓ o j/k: mover · 1-7: elegir · enter: entrar · q: salir"
	strTooSmallFmt = "La terminal es muy pequeña (%d×%d).\nSe necesita al menos %d×%d."
)

// Lista de módulos (modo Lecciones).
const (
	strModulesTitle   = "Lecciones · elige un módulo"
	strModulesHelp    = "↑/↓: mover · enter: empezar · esc: volver"
	strModuleCountFmt = "%d ejercicios"
	strModuleEmpty    = "próximamente"
	strModuleLocked   = "Completa el módulo anterior con rango C o superior para desbloquearlo."
	strModuleNoItems  = "Este módulo todavía no tiene ejercicios."
	strLockMark       = "[bloqueado]"
	strModuleBestFmt  = "mejor: %s"
)

// Textos comunes a las pantallas de ejercicio.
const (
	strCorrect        = "✔ ¡Correcto!"
	strWrong          = "✘ Incorrecto"
	strTimeout        = "⏱ Se acabó el tiempo"
	strSource         = "Para repasar:"
	strHintFmt        = "Pista %d: %s"
	strHelpHint       = " · F1: pista"
	strHelpBack       = " · esc: salir del módulo"
	strHelpNext       = "enter: siguiente · esc: salir del módulo"
	strPointsFmt      = "+%d pts"
	strPointsComboFmt = "+%d pts (combo ×%.1f)"
)

// Pantalla de quiz.
const (
	strQuizInputHolder  = "escribe tu respuesta"
	strQuizHelpChoice   = "↑/↓ o 1-9: elegir · enter: responder"
	strQuizHelpMulti    = "↑/↓: mover · espacio o 1-9: marcar · enter: responder"
	strQuizHelpText     = "enter: responder"
	strQuizTryAgainFmt  = "Incorrecto. Te queda %d intento."
	strQuizAnswerFmt    = "Respuesta correcta: %s"
	strQuizErrorFmt     = "Error al evaluar la pregunta: %v"
	strQuizTrue         = "Verdadero"
	strQuizFalse        = "Falso"
	strQuizAnswerJoiner = ", "
)

// Pantalla de práctica.
const (
	strPracticePreparing   = "Preparando el sandbox…"
	strPracticeRunning     = "ejecutando…"
	strPracticeAttemptsFmt = "Intentos: %d"
	strPracticeBackendFmt  = "Sandbox: %s"
	strPracticeDirWarning  = "⚠ Sandbox sin aislamiento: los comandos corren en tu equipo con tus permisos. Instala bubblewrap o usa --sandbox container (make image)."
	strPracticeNotYetFmt   = "Todavía no. Te quedan %d intentos."
	strPracticeNotYetOne   = "Todavía no. Te queda 1 intento."
	strPracticeSolved      = "✔ ¡Resuelto!"
	strPracticeOutOfTries  = "✘ Se acabaron los intentos"
	strPracticeSolutionFmt = "Una solución: %s"
	strPracticeTimedOutFmt = "⏱ El comando tardó más de %s y se detuvo. La terminal se reinició (las variables se perdieron)."
	strPracticeRestarted   = "La terminal se cerró y se abrió una nueva (las variables se perdieron)."
	strPracticeTruncated   = "… (salida recortada)"
	strPracticeReset       = "── sandbox reiniciado: archivos como al inicio del ejercicio ──"
	strPracticeSetupErrFmt = "No se pudo preparar el sandbox: %v"
	strPracticeSetupHelp   = "enter: saltar el ejercicio · esc: salir del módulo"
	strPracticeRunErrFmt   = "Error del sandbox: %v"
	strPracticeHelp        = "enter: ejecutar · ↑/↓: historial · PgUp/PgDn: desplazar · F2: reiniciar · F3: árbol · Ctrl-L: limpiar"
	strPracticeInputHolder = "escribe un comando"

	strInteractiveDone = "(programa interactivo terminado)"

	strEditorPlaceholder    = "escribe aquí tu script"
	strEditorFileFmt        = "Archivo: %s"
	strEditorRunning        = "Corriendo las pruebas…"
	strEditorHelpFmt        = "F5: guardar y probar · F4: abrir en %s · tab: sangría"
	strEditorNoShebang      = "✘ La primera línea del script debe ser el shebang (#!/bin/bash)."
	strEditorTestFmt        = "Prueba %d: %s"
	strEditorArgsFmt        = "argumentos: %s"
	strEditorStdinFmt       = "entrada: %s"
	strEditorNoArgs         = "sin argumentos"
	strEditorGotFmt         = "  tu script mostró: %s"
	strEditorNoOutput       = "(nada)"
	strEditorExpectedFmt    = "se esperaba que mostrara «%s»"
	strEditorBadFormat      = "la salida no tiene el formato que pide el ejercicio"
	strEditorExitFmt        = "terminó con código %d, se esperaba %d"
	strEditorFileMissingFmt = "no se creó el archivo %s"
	strEditorFileWrongFmt   = "%s quedó con: %s"
	strEditorTimedOut       = "tardó demasiado (¿un ciclo que nunca termina, o un read sin entrada?)"
	strEditorSolution       = "Una solución:"

	strPatternNote   = "En este ejercicio el comando no se ejecuta: se compara con las respuestas esperadas."
	strPatternNotRun = "(no se ejecuta: en este ejercicio solo se revisa el comando que escribes)"
	strPatternHelp   = "enter: responder · ↑/↓: historial · Ctrl-L: limpiar"
)

// Mensajes de guard: se muestran en vez de ejecutar el comando.
const (
	strGuardPrivilege      = "🛡 %s no se ejecuta aquí: sirve para actuar como administrador (root), y en esta práctica no hace falta. En un equipo real úsalo solo cuando sepas exactamente qué hará el comando."
	strGuardPower          = "🛡 %s apagaría o reiniciaría el equipo. En un servidor compartido eso afecta a todas las personas conectadas."
	strGuardMkfs           = "🛡 %s formatea un disco y borra todo lo que tiene. No se ejecuta en esta práctica."
	strGuardDeviceWrite    = "🛡 «%s» escribiría directo sobre un dispositivo y podría destruir un disco completo."
	strGuardForkBomb       = "🛡 Eso es una fork bomb: una función que se copia a sí misma sin fin hasta agotar el sistema."
	strGuardOutsidePath    = "🛡 %s está fuera de tu directorio de práctica. Aquí solo se modifican archivos dentro de ~."
	strGuardHomeWipe       = "🛡 «%s» borraría tu home completo. Revisa bien la ruta antes de usar rm -r."
	strGuardUnverifiable   = "🛡 No se puede comprobar a qué ruta apunta «%s» antes de ejecutarlo, y este sandbox no está aislado. Escribe la ruta directamente."
	strGuardForeignProcess = "🛡 «%s» podría alcanzar programas tuyos fuera de la práctica (este sandbox no está aislado). Usa kill con el PID de un proceso del ejercicio, o %%1 para el primer trabajo en segundo plano."
	strGuardPenaltyNote    = " (−10 puntos, solo la primera vez)"
)

// HUD.
const (
	strHUDExerciseFmt     = "%s · %d de %d"
	strHUDExerciseOpenFmt = "%s · #%d"
	strHUDTotalFmt        = "(quedan %s)"
	strHUDTimerFmt        = "⏱ %ds"
	strHUDScoreFmt        = "%d pts"
	strHUDComboFmt        = "×%.1f"
)

// Pantalla de resultados.
const (
	strResultsTitleFmt   = "Resultados · %s"
	strResultsRank       = "Rango"
	strResultsScore      = "Puntaje"
	strResultsScoreFmt   = "%d de %d (%.0f%%)"
	strResultsTime       = "Tiempo"
	strResultsAccuracy   = "Precisión"
	strResultsAccFmt     = "%d de %d al primer intento (%.0f%%)"
	strResultsMaxCombo   = "Combo máx."
	strResultsByModule   = "Por módulo"
	strResultsReview     = "Temas a repasar"
	strResultsNoReview   = "¡Nada que repasar! Todo al primer intento."
	strResultsMissFmt    = "%d fallo"
	strResultsMissesFmt  = "%d fallos"
	strResultsHelp       = "enter: volver a los módulos"
	strResultsHelpMenu   = "enter: volver al menú"
	strResultsScroll     = "↑/↓ o PgUp/PgDn: ver más"
	strResultsNewRecord  = "★ ¡Nuevo récord!"
	strResultsModuleBest = "★ Tu mejor resultado en este módulo"
	strResultsPosFmt     = "Lugar %d en el ranking"
	strResultsUnlockFmt  = "🔓 Desbloqueaste: %s"
	strResultsOfficial   = "Este es tu rango oficial del examen final"
	strResultsSaveErrFmt = "⚠ No se pudo guardar el progreso: %v"
)

// Modos de juego y ranking.
const (
	strModeTimeAttackFmt = "Contrarreloj %d s"
	strTimeAttackTitle   = "Contrarreloj · elige la duración"
	strTimeAttackItemFmt = "%d segundos"
	strChoiceHelp        = "↑/↓ o número: elegir · enter: empezar · esc: volver"
	strExamLocked        = "El examen final se desbloquea al terminar el módulo 9 con rango C o superior."
	strModeEmpty         = "Todavía no hay ejercicios disponibles para este modo."
	strLeaderboardTitle  = "Ranking"
	strLeaderboardEmpty  = "Todavía no hay puntajes en este modo."
	strColPoints         = "Puntos"
	strColRank           = "Rango"
	strColPercent        = "Porc."
	strColTime           = "Tiempo"
	strColDate           = "Fecha"
	strLeaderboardHelp   = "←/→: cambiar de modo · esc: volver"
	strStorageWarnFmt    = "⚠ %v"
)

// Pantalla «Acerca de».
const (
	strAboutIntro        = "Una aplicación de terminal para aprender y practicar la terminal GNU/Linux: lecciones por módulos, ejercicios con comandos reales en un sandbox, contrarreloj, puntos y rangos."
	strAboutCreditsTitle = "Créditos"
	strAboutCredits      = "El temario sigue el curso «Terminal GNU/Linux Nivel cero» del Laboratorio de Supercómputo y Visualización en Paralelo (LSVP) de la UAM Iztapalapa. Las preguntas, explicaciones y archivos de práctica se escribieron para esta aplicación a partir de ese temario."
	strAboutCourseURL    = "https://lsvp-uami.github.io/mdbook-curso-linux-0/"
	strAboutLicense      = "El material del curso se publica bajo la licencia Apache-2.0 en github.com/LSVP-UAMI/mdbook-curso-linux-0."
	strAboutSandbox      = "Sandbox"
	strAboutProgress     = "Progreso"
	strAboutNoConfig     = "no se guarda (sin directorio de configuración)"
	strAboutHelp         = "esc: volver"
)
