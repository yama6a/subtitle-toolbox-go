// Package markup holds the helpers for core markup, the HTML-like inline tags of cue lines.
//
// A cue line such as `<i>Fish &amp; Chips</i>` holds text with &, < and > escaped, and tags such as <b>, <i>,
// <font color="#ff0000">, <v Name> and word timestamps such as <00:01:02.500>. A tag is a "<" that is not followed by
// white space, up to the next ">". docs/markup.md lists the full tag set and the escaping rule.
package markup
