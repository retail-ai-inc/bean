<div id="top"></div>

# BEAN (豆)
A web framework written in GO on-top of `echo` to ease your application development.
- [BEAN (豆)](#bean-豆)
  - [How to use](#how-to-use)
    - [Initialize a project](#initialize-a-project)
    - [Upgrade the framework code inside a project](#upgrade-the-framework-code-inside-a-project)
  - [Styling](#styling)
    - [Comment](#comment)
  - [Do’s and Don’ts](#dos-and-donts)
    - [Context](#context)
    - [Pointer](#pointer)

## How to use
### Initialize a project
1. Install the package by
```
go install github.com/retail-ai-inc/bean@latest
```
2. Create a project directory
```
mkdir my_project && cd my_project
```
3. Initialize the project using bean by
```
bean init my_project
```
or
```
bean init github.com/me/my_project  // if the project will be published.
```
### Upgrade the framework code inside a project
1. Update the bean to latest version
```
go install github.com/retail-ai-inc/bean@latest
```
2. Navigate to the project directory
```
cd my_project
```
3. Run the `upgrade` command
```
bean upgrade
```

## Styling
### Comment
Please add the following header in every files.
```
// Copyright The RAI Inc.
// The RAI Authors
```
Please use `//` for any comment.
```
// This is a single line comment.

// This is a
// multiline comment.
```
For some special message, please add appropiate TAG at the beginning of the comment.
```
// IMPORTANT: This is super important comment.
// WARN:
// TODO:
// FIX:
// ISSUE:
```

## Do’s and Don’ts
### Context
Do not use `c.Get` and `c.Set` in `Service` and `Repository` layer to avoid confusion, because `c.Get` and `c.Set` is using hardcoded variable name for storing the data. Instead of storing the variable inside the `echo.Context`, just pass it explicitly through function parameters.

### Pointer
```
As in all languages in the C family, everything in Go is passed by value. That is, a function
always gets a copy of the thing being passed, as if there were an assignment statement assigning
the value to the parameter. For instance, passing an int value to a function makes a copy of the
int, and passing a pointer value makes a copy of the pointer, but not the data it points to.
```
For complicated object, pointer should be used as parameter instead of values to reduce the usage of copying the whole object. ref: [https://go.dev/doc/faq#pass_by_value](https://go.dev/doc/faq#pass_by_value)

## Template Development

### Replacement Directive

In `.go` file, you can wrap content with `/**#bean*/` and `/*#bean.replace(<Replacement>)**/`. Bean will replace the content with the replacement.
for example, when content in template file is 
```text
/**#bean*/"demo/framework/dbdrivers"/*#bean.replace("{{ .PkgPath }}/framework/dbdrivers")**/
```
Bean will generate:
```text
"{{ .PkgPath }}/framework/dbdrivers"
```

`/**#bean*/` should in the head of line. Because `go fmt` will reorder it.
for example
before `go fmt`
```text
ierror /**#bean*/ "demo/framework/internals/error" /*#bean.replace(ierror "{{ .PkgPath }}/framework/internals/error")**/
```
after `go fmt`
```text
ierror "demo/framework/internals/error" /**#bean*/  /*#bean.replace(ierror "{{ .PkgPath }}/framework/internals/error")**/
```
In this situation, you should write it:
```text
/**#bean*/ ierror "demo/framework/internals/error" /*#bean.replace(ierror "{{ .PkgPath }}/framework/internals/error")**/
```

### Template File For Hidden File

goembed don't support the file which start with `.`, so we can name it with prefix `bean-dot`
for example, we want to make `.gitignore` as template, we can name it as `bean-dot.gitignore`, Bean init command will rename `bean-dot.gitignore` to `.gitignore` 