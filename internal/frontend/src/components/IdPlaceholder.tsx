// IdPlaceholder renders the literal "<id>" placeholder used in CLI command hints.
// It exists so react-i18next <Trans> can map a self-closing <id/> tag to a real
// React text node, avoiding the HTML-entity decoding problem of &lt;id&gt;.
export function IdPlaceholder() {
  return <>{"<id>"}</>;
}
