# Kuery (All in one utility for Golang projects)

Wrapper for many libraries to make it easier to work with go projects.

## Install

This package requires Go(v1.20+).

```shell
go get github.com/vukyn/kuery/v2
```

## Usage

### conv pkg (Conversion)

#### Datetime:

Work with datetime conversion.

-   Example:

```go
    unix := 1718298000000
    unixStr := conv.FormatUnixToString(unix, t.YYYY_MM_DD)
    fmt.Println(unixStr) // 2024-02-29
```

| Format              | Description                     |
| ------------------- | ------------------------------- |
| RFC822              | "02 Jan 06 15:04 MST"           |
| Kitchen             | "3:04PM"                        |
| UnixDate            | "Mon Jan \_2 15:04:05 MST 2006" |
| HH_MM_SS            | "15:04:05"                      |
| YYYY_MM_DD          | "2006-01-02"                    |
| DD_MM_YYYY          | "02-01-2006"                    |
| YYYY_MM_DD_HH_MM_SS | "2006-01-02 15:04:05"           |
| DD_MM_YYYY_HH_MM_SS | "02-01-2006 15:04:05"           |

#### Interface:

Work with interface conversion.

-   Example:

ReadInterface read value from interface. Return default value if key not exists or empty value.

```go
    defaultAge := 18
    data := map[string]interface{}{
        "name": "Hello",
        "age": 0,
        "gender": "",
    }

    // Read data from interface with given key
    name := conv.ReadInterface(data, "name", "")

    // Read data from interface with given key and return default value if not found
    age1 := conv.ReadInterface(data, "age", defaultAge, false)
    gender1 := conv.ReadInterface(data, "gender", "Male", false)

    // Read data from interface with given key and default value if not found and check if value is empty
    age2 := conv.ReadInterface(data, "age", defaultAge, true)
    gender2 := conv.ReadInterface(data, "gender", "Male", true)

    fmt.Printf("%s-%d-%s-%d-%s\n", name, age1, gender1, age2, gender2) // Hello-0--18-Male
```

DoubleSlice help deals with variadic functions in Go.

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
