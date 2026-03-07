# Agents Development Guidelines

These are the rules for agents to strictly adhere to when developing code inside this project.

## Project Description

We are building a terminal application that is comparable to neovim, but with advanced A.I. development tools directly integrated into the eco-system as a first-class citizen.

## Minimal Dependencies

We follow a minimal dependency architecture, and develop every part from the ground up to follow a strict core-philosophy.

## Core Philosophy

**Performance is not negotiable**

If there is still performance on left on the table, no matter the complication, then we are not done yet.

**Everything is io.ReadWriteCloser**

Every object, no matter what, should implement io.ReadWriteCloser, and that is the only way things should communicate.

That means to send data from one thing to another we use the io primitives, like `io.Copy`, `io.MultiWriter`, `io.TeeReader`, etc.

**Cap 'n Proto**

We use Cap 'n Proto and develop a single (mono-type) "transport" object, that allows us to easily "serialize" and "deserialize" data, benefitting from the unique property of Cap 'n Proto to basically make this "free" of overhead. This makes the `io.ReadWriteCloser` system viable.

## Coding style

Each "thing" should be an object with methods. We don't like loose functions. A typical object usually follows a pattern like below.
Since all objects should be implementing `io.ReadWriteCloser` there should be no need to make an object much bigger than that.
Never have two objects in the same file.

> !NOTE
> Everything in examples like this is important and deliberate.
> For example, we really do not like single character variable names.
> We also want code to be spaced out correctly vertically, so put newlines between groupings.
> A block should always have a newline and be separate from the code above and below it.

```go
package packagename

/*
ObjectName is something descriptive.
It also has a reason why it was implemented.
*/
type ObjectName struct {
    err error
}

/*
opts configures ObjectName with options.
*/
type opts func(*ObjectName)

/*
NewObjectName instantiates a new ObjectName.
It also has a reason for being instantiated.
*/
func NewObjectName(opts ...opts) *ObjectName {

}

/*
Read implements the io.Reader interface.
*/
func (objectName *ObjectName) Read(p []byte) (n int, err error) {
    return
}

/*
Write implements the io.Write interface.
*/
func (objectName *ObjectName) Write(p []byte) (n int, err error) {
    return
}

/*
Close implements the io.Reader interface.
*/
func (objectName *ObjectName) Close() (err error) {
    return
}
```

> !NOTE
> A final remark on code quality.
> Less is always more, refactoring is not optional. If it can be done with less code, do it with less code.
> If you see something that is not yours that can be done with less code, refactor it.
> However, if less code means less performance, then always choose performance.
> We like clever code, readability is for amateurs.

## Testing

We always use Goconvey for testing, and tests follow a simple structure. Every file should have a test file that mirrors its structure. So each file has an accompanying `_test.go` file, with functions that mirror the code's methods, prefix by `Test`.
We follow a nested BDD approach `Given something`, `It should do something`.
Always add benchmarks too, so we can measure performance.

Make sure tests and benchmarks are truly meaningful, don't test for testing's sake, make sure it truly validates the code.