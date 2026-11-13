# Upgrade to 4.0

Version 4.0 moves the module path to `/v4`. This page lists each break with the old call and the new call.

```bash
go get github.com/yama6a/subtitle-toolbox-go/v4@latest
```

## The 3.x call and the 4.0 call

| 3.x | 4.0 |
|:--- |:--- |
| `errors.As(err, &serr)` with a `*json.SyntaxError` matched the cause of a library JSON parse error | it no longer matches. The cause is an internal type. Read the message of the `*ParseError` instead, which holds the PHP text such as `The content is not valid JSON: Syntax error.` |
