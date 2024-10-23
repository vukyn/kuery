# Kuery (All in one utility for Golang projects)

Wrapper for many libraries to make it easier to work with go projects.

## Install

This package requires Go(v1.20+).

```shell
go get github.com/vukyn/kuery/v2
```

## Usage

### conv pkg (Conversion)

#### Interface:

Work with interface conversion.

-   Example:

IsZero check if interface is zero value.

```go
    fmt.Println(conv.IsZero(0)) // true
    fmt.Println(conv.IsZero(1)) // false
    fmt.Println(conv.IsZero("")) // true
    fmt.Println(conv.IsZero("Hello")) // false
    fmt.Println(conv.IsZero(nil)) // true
    fmt.Println(conv.IsZero([]int{})) // true
    fmt.Println(conv.IsZero([]int{1})) // false
    fmt.Println(conv.IsZero(map[string]interface{}{})) // true
    fmt.Println(conv.IsZero(map[string]interface{}{"name": "Hello"})) // false
```

#### Parse:

Parse from number/string to array of number/string and vice versa.

```go
    fmt.Println(conv.NumberToString(1)) // "1"
    fmt.Println(conv.ArrayNumberToString([]int{1, 2, 3}, ",")) // "1,2,3"
    fmt.Println(conv.StringToArrayNumber[int64]("1,2,3", ",")) // "[1 2 3]"
    fmt.Println(conv.ArrayStringToString([]string{"a", "b", "c"}, ",")) // "a,b,c"
    fmt.Println(conv.StringToArrayString("a,b,c", ",")) // "[a b c]"
    fmt.Println(conv.ToPointer(1)) // "0x140000a6018"
```
