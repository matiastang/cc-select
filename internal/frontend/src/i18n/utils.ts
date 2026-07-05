export function updateDocumentLang(language: string) {
  document.documentElement.lang = language === "zh" ? "zh-CN" : language;
}

export function updateDocumentTitle(title: string) {
  document.title = title;
}
