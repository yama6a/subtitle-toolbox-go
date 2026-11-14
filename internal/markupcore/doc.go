// Package markupcore holds the core markup helpers that package markup and the format packages share.
//
// Patterns without the PCRE /u flag treat VT as white space, so they spell white space as [ \t\n\r\f\v]
// instead of \s. Trimming uses TrimSet, which keeps U+00A0.
package markupcore
