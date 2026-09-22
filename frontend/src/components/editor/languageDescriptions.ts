import {
  LanguageDescription,
  LanguageSupport,
  StreamLanguage,
  type StreamParser,
} from "@codemirror/language";

/**
 * Syntax highlighters for fenced code blocks.
 *
 * `@codemirror/language-data` registers every language CodeMirror supports —
 * 143 of them. Each registration is a separate dynamic import, so an unused
 * language still ships a deployable chunk: the full list measured ~1000 kB
 * across 111 files, over a third of the whole admin bundle, for languages the
 * picker in `codeLanguages.tsx` cannot even select.
 *
 * This module registers only what that picker offers. The descriptors are
 * copied verbatim from upstream, so names, aliases and loaders behave exactly
 * as before: a fence tagged js, csharp or bash resolves to the same parser it
 * always did.
 *
 * Adding a language to the picker therefore means adding its descriptor here,
 * otherwise the fence falls back to plain text.
 */

function legacy(parser: StreamParser<unknown>) {
  return new LanguageSupport(StreamLanguage.define(parser));
}

function sql(dialectName: "StandardSQL") {
  return import("@codemirror/lang-sql").then((m) =>
    m.sql({ dialect: m[dialectName] }),
  );
}

export const CODE_LANGUAGE_DESCRIPTIONS: readonly LanguageDescription[] = [
  LanguageDescription.of({
    name: "C",
    extensions: ["c", "h", "ino"],
    load() {
      return import("@codemirror/lang-cpp").then((m) => m.cpp());
    },
  }),
  LanguageDescription.of({
    name: "C++",
    alias: ["cpp"],
    extensions: ["cpp", "c++", "cc", "cxx", "hpp", "h++", "hh", "hxx"],
    load() {
      return import("@codemirror/lang-cpp").then((m) => m.cpp());
    },
  }),
  LanguageDescription.of({
    name: "CSS",
    extensions: ["css"],
    load() {
      return import("@codemirror/lang-css").then((m) => m.css());
    },
  }),
  LanguageDescription.of({
    name: "Go",
    extensions: ["go"],
    load() {
      return import("@codemirror/lang-go").then((m) => m.go());
    },
  }),
  LanguageDescription.of({
    name: "HTML",
    alias: ["xhtml"],
    extensions: ["html", "htm", "handlebars", "hbs"],
    load() {
      return import("@codemirror/lang-html").then((m) => m.html());
    },
  }),
  LanguageDescription.of({
    name: "Java",
    extensions: ["java"],
    load() {
      return import("@codemirror/lang-java").then((m) => m.java());
    },
  }),
  LanguageDescription.of({
    name: "JavaScript",
    alias: ["ecmascript", "js", "node"],
    extensions: ["js", "mjs", "cjs"],
    load() {
      return import("@codemirror/lang-javascript").then((m) => m.javascript());
    },
  }),
  LanguageDescription.of({
    name: "JSON",
    alias: ["json5"],
    extensions: ["json", "map"],
    load() {
      return import("@codemirror/lang-json").then((m) => m.json());
    },
  }),
  LanguageDescription.of({
    name: "JSX",
    extensions: ["jsx"],
    load() {
      return import("@codemirror/lang-javascript").then((m) =>
        m.javascript({ jsx: true }),
      );
    },
  }),
  LanguageDescription.of({
    name: "Markdown",
    extensions: ["md", "markdown", "mkd"],
    load() {
      return import("@codemirror/lang-markdown").then((m) => m.markdown());
    },
  }),
  LanguageDescription.of({
    name: "PHP",
    extensions: ["php", "php3", "php4", "php5", "php7", "phtml"],
    load() {
      return import("@codemirror/lang-php").then((m) => m.php());
    },
  }),
  LanguageDescription.of({
    name: "Python",
    extensions: ["BUILD", "bzl", "py", "pyw"],
    filename: /^(BUCK|BUILD)$/,
    load() {
      return import("@codemirror/lang-python").then((m) => m.python());
    },
  }),
  LanguageDescription.of({
    name: "Rust",
    extensions: ["rs"],
    load() {
      return import("@codemirror/lang-rust").then((m) => m.rust());
    },
  }),
  LanguageDescription.of({
    name: "SCSS",
    extensions: ["scss"],
    load() {
      return import("@codemirror/lang-sass").then((m) => m.sass());
    },
  }),
  LanguageDescription.of({
    name: "SQL",
    extensions: ["sql"],
    load() {
      return sql("StandardSQL");
    },
  }),
  LanguageDescription.of({
    name: "TSX",
    extensions: ["tsx"],
    load() {
      return import("@codemirror/lang-javascript").then((m) =>
        m.javascript({ jsx: true, typescript: true }),
      );
    },
  }),
  LanguageDescription.of({
    name: "TypeScript",
    alias: ["ts"],
    extensions: ["ts", "mts", "cts"],
    load() {
      return import("@codemirror/lang-javascript").then((m) =>
        m.javascript({ typescript: true }),
      );
    },
  }),
  LanguageDescription.of({
    name: "XML",
    alias: ["rss", "wsdl", "xsd"],
    extensions: ["xml", "xsl", "xsd", "svg"],
    load() {
      return import("@codemirror/lang-xml").then((m) => m.xml());
    },
  }),
  LanguageDescription.of({
    name: "YAML",
    alias: ["yml"],
    extensions: ["yaml", "yml"],
    load() {
      return import("@codemirror/lang-yaml").then((m) => m.yaml());
    },
  }),
  LanguageDescription.of({
    name: "C#",
    alias: ["csharp", "cs"],
    extensions: ["cs"],
    load() {
      return import("@codemirror/legacy-modes/mode/clike").then((m) =>
        legacy(m.csharp),
      );
    },
  }),
  LanguageDescription.of({
    name: "Dart",
    extensions: ["dart"],
    load() {
      return import("@codemirror/legacy-modes/mode/clike").then((m) =>
        legacy(m.dart),
      );
    },
  }),
  LanguageDescription.of({
    name: "diff",
    extensions: ["diff", "patch"],
    load() {
      return import("@codemirror/legacy-modes/mode/diff").then((m) =>
        legacy(m.diff),
      );
    },
  }),
  LanguageDescription.of({
    name: "Dockerfile",
    filename: /^Dockerfile$/,
    load() {
      return import("@codemirror/legacy-modes/mode/dockerfile").then((m) =>
        legacy(m.dockerFile),
      );
    },
  }),
  LanguageDescription.of({
    name: "HTTP",
    load() {
      return import("@codemirror/legacy-modes/mode/http").then((m) =>
        legacy(m.http),
      );
    },
  }),
  LanguageDescription.of({
    name: "Kotlin",
    extensions: ["kt", "kts"],
    load() {
      return import("@codemirror/legacy-modes/mode/clike").then((m) =>
        legacy(m.kotlin),
      );
    },
  }),
  LanguageDescription.of({
    name: "Lua",
    extensions: ["lua"],
    load() {
      return import("@codemirror/legacy-modes/mode/lua").then((m) =>
        legacy(m.lua),
      );
    },
  }),
  LanguageDescription.of({
    name: "PowerShell",
    extensions: ["ps1", "psd1", "psm1"],
    load() {
      return import("@codemirror/legacy-modes/mode/powershell").then((m) =>
        legacy(m.powerShell),
      );
    },
  }),
  LanguageDescription.of({
    name: "Properties files",
    alias: ["ini", "properties"],
    extensions: ["properties", "ini", "in"],
    load() {
      return import("@codemirror/legacy-modes/mode/properties").then((m) =>
        legacy(m.properties),
      );
    },
  }),
  LanguageDescription.of({
    name: "R",
    alias: ["rscript"],
    extensions: ["r", "R"],
    load() {
      return import("@codemirror/legacy-modes/mode/r").then((m) => legacy(m.r));
    },
  }),
  LanguageDescription.of({
    name: "Ruby",
    alias: ["jruby", "macruby", "rake", "rb", "rbx"],
    extensions: ["rb"],
    filename: /^(Gemfile|Rakefile)$/,
    load() {
      return import("@codemirror/legacy-modes/mode/ruby").then((m) =>
        legacy(m.ruby),
      );
    },
  }),
  LanguageDescription.of({
    name: "Scala",
    extensions: ["scala"],
    load() {
      return import("@codemirror/legacy-modes/mode/clike").then((m) =>
        legacy(m.scala),
      );
    },
  }),
  LanguageDescription.of({
    name: "Shell",
    alias: ["bash", "sh", "zsh"],
    extensions: ["sh", "ksh", "bash"],
    filename: /^PKGBUILD$/,
    load() {
      return import("@codemirror/legacy-modes/mode/shell").then((m) =>
        legacy(m.shell),
      );
    },
  }),
  LanguageDescription.of({
    name: "Swift",
    extensions: ["swift"],
    load() {
      return import("@codemirror/legacy-modes/mode/swift").then((m) =>
        legacy(m.swift),
      );
    },
  }),
  LanguageDescription.of({
    name: "TOML",
    extensions: ["toml"],
    load() {
      return import("@codemirror/legacy-modes/mode/toml").then((m) =>
        legacy(m.toml),
      );
    },
  }),
  LanguageDescription.of({
    name: "Vue",
    extensions: ["vue"],
    load() {
      return import("@codemirror/lang-vue").then((m) => m.vue());
    },
  }),
];
