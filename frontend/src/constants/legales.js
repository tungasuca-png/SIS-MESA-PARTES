// Documentos legales del registro manual. Los publica la institución; aquí
// solo se enlazan (VITE_TERMINOS_URL, VITE_PRIVACIDAD_URL en frontend/.env).
// La versión aceptada se envía al backend y queda registrada junto con la
// fecha de aceptación. "no-publicada" deja constancia de que los documentos
// todavía no estaban configurados.
export const TERMINOS_URL = (import.meta.env.VITE_TERMINOS_URL || "").trim();
export const PRIVACIDAD_URL = (import.meta.env.VITE_PRIVACIDAD_URL || "").trim();
export const TERMINOS_VERSION = (import.meta.env.VITE_TERMINOS_VERSION || "").trim() || "no-publicada";
