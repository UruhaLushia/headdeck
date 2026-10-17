import { Highlight, type PrismTheme } from "prism-react-renderer";

const theme: PrismTheme = {
  plain: { color: "var(--foreground)", backgroundColor: "transparent" },
  styles: [
    { types: ["property"], style: { color: "light-dark(#0550ae, #79c0ff)" } },
    { types: ["string"], style: { color: "light-dark(#116329, #a5d6a7)" } },
    { types: ["number"], style: { color: "light-dark(#953800, #ffa657)" } },
    { types: ["boolean", "null"], style: { color: "light-dark(#6639ba, #d2a8ff)" } },
    { types: ["punctuation", "operator"], style: { color: "var(--muted)" } },
  ],
};

export function JsonCode({ code }: { code: string }) {
  return (
    <Highlight code={code} language="json" theme={theme}>
      {({ style, tokens, getLineProps, getTokenProps }) => (
        <pre
          style={style}
          className="font-mono text-sm leading-relaxed whitespace-pre-wrap wrap-anywhere select-text"
        >
          <code>
            {tokens.map((line, index) => (
              <span key={index} {...getLineProps({ line })}>
                {line.map((token, key) => (
                  <span key={key} {...getTokenProps({ token })} />
                ))}
                {index < tokens.length - 1 ? "\n" : ""}
              </span>
            ))}
          </code>
        </pre>
      )}
    </Highlight>
  );
}
