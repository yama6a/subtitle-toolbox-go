// Package markup holds the helpers for core markup, the HTML-like inline tags of cue lines.
//
// A cue line such as `<i>Fish &amp; Chips</i>` holds text with &, < and > escaped, and tags such as <b>, <i>,
// <font color="#ff0000"> and <v Name>. A tag is a "<" that is not followed by white space, up to the next ">".
// The lines of a cue hold no word timestamps, because model.Cue.Words holds them. docs/markup.md lists the full tag
// set and the escaping rule.
package markup
