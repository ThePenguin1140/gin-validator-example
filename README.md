# Gin Validation
In recent years developer experience is becoming a more prioritized aspect of API design. Ensuring that developers can quickly develop on your platform can help achieve a variety of goals like decrease support burden, increasing stickiness and increasing the speed with which third party developers can integrate with your system. One way to provide a good experience, and protect your system, is validating your input data (e.g., query parameters or request bodies). But that's why you're here so lets move on.
I'm sure we've all experienced working with systems where error messages were either not very helpful or downright absent. So it's not just enough to validate; The best APIs return useful messages that help the caller fix their mistakes and move on quickly.
Luckily, gin-gonic/gin, a high performance golang powered API framework leverages the popular go-playground/validator library and lets you configure validation straight in your struct tags. 
Unfortunately, knowing how to configure these validators, extend/customize them and return useful errors is more difficult that I'd like. I personally found documentation lacking and it not incredibly obvious how that all worked. That kept me from creating the smooth API experience that I wanted. 
After spelunking through documentation and the gin source code I've learned a few things that I think will make it easier for you to configure solid validation rules and enhance the usability of your API.

## Let's get to it
To configuring the default gin validator you use the `binding` struct tag. This will perform data validation when you call `ShouldBind` ,`MustBind` or any of their derivatives. It's important to note that marshalling happens first of course so if there is a marshalling error (e.g., time cannot be parsed, or a `string` is passed in instead of an `int`) then the validation will not occur and you'll get a `json.MarshallingTypeError`or `time.ParseError` instead. 

As an example we're going to create an endpoint that returns the time between two dates. 
```go
package main

import (
	"fmt"
	"net/http"
	"github.com/gin-gonic/gin"
)

type params struct {
	Start *string `json:"start" form:"start" binding:"required_without=End,omitempty,datetime=2006-01-02T15:04:05Z07:00"`
	End *string `json:"end" form:"end" binding:"required_without=Start,omitempty,datetime=2006-01-02T15:04:05Z07:00"`
}

func main() {
	r := gin.Default()
	r.GET("/", func(c *gin.Context) {
		params := params{}
		if err := c.ShouldBind(&params); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"errors": fmt.Sprintf("%v", err)})
			return
		}
		c.JSON(http.StatusOK, fmt.Sprintf("%v", params))
		return
	})
	r.Run() 
}
```

Lets decompose this:
`form:` will bind this struct field to a query parameter and the `binding:` tags configure the validaiton aspect. You can list as many validators in the binding string as you like as long as they're compatible with each other and the data type you're validating. The inclusion of the `required_without` does what you would expect it and avoids a series of if statemens checking the same thing. 
If we run this without any query parameters we get the following response:
```json
{
  "errors": "Key: 'params.Start' Error:Field validation for 'Start' failed on the 'required_without' tag\nKey: 'params.End' Error:Field validation for 'End' failed on the 'required_without' tag"
}
```

It works … but it's not very readable, so lets address that!
Gin will return `validator.ValidationErrors` (read a bit more [here](https://pkg.go.dev/github.com/go-playground/validator/v10#hdr-Validation_Functions_Return_Type_error)) so via a type assertion we can access pretty detailed information about which field didn't pass validation, why and even access the specific parameters passed to the validator. That allows us to create a helper function like this:
```go
func parseError(err error) []string {
	if validationErrs, ok := err.(validator.ValidationErrors); ok {
		errorMessages := make([]string, len(validationErrs))
		for i, e := range validationErrs {
			switch(e.Tag()) {
			case "required_without":
				errorMessages[i] = fmt.Sprintf("The field %s is required if %s is not supplied", e.Field(), e.Param())
			}
		}
		return errorMessages
	} else if marshallingErr, ok := err.(*json.UnmarshalTypeError); ok {
		return []string{fmt.Sprintf("The field %s must be a %s", marshallingErr.Field, marshallingErr.Type.String())}
	}
	return nil
}
```

This can be enhanced with additional switch statements to make clear and understandable error messages for each tag that's in use!
Adding that to our code above will change the output to a much nicer series of messages:
```json
{
	"errors": [
		"The field Start is required if End is not supplied",
		"The field End is required if Start is not supplied"
	]
}
```

To finalize our goal of creating an endpoint that returns the duration between two dates we'll add another struct:
```go
type DateRange struct {
	Start time.Time `form:"start" binding:"omitempty,lt|ltfield=End"`
	End time.Time `form:"end" binding:"omitempty,gt|gtfield=Start"`
}
```
What's special here is that we're actually going to be unmarshalling to `time.Time` which allows us to do range comparisons on the times provided. Via the binding validators we can ensure that the dates are always a valid range but also don't fail if one of them is left blank. The rules in the above example make sure that the following rules are always true:
* `start < end`
* `start < now` if `end` is not supplied
* `end > now` if `start` is not supplied 

And since we already built a helper function we can just add the following switch cases to make sure that ever validator has a readable response associated with it!
```go
func parseError(err error) []string {
	if validationErrs, ok := err.(validator.ValidationErrors); ok {
		errorMessages := make([]string, len(validationErrs))
		for i, e := range validationErrs {
			// workaround to the fact that the `gt|gtfield=Start` gets passed as an entire tag for some reason
			// https://github.com/go-playground/validator/issues/926
			tag := strings.Split(e.Tag(),"|")[0] 
			switch(tag) {
			case "required_without":
				errorMessages[i] = fmt.Sprintf("The field %s is required if %s is not supplied", e.Field(), e.Param())
			case "lt", "ltfield":
				param := e.Param()
				if param == "" {
					param = time.Now().Format(time.RFC3339)
				}
				errorMessages[i] = fmt.Sprintf("The field %s is must be less than %s", e.Field(), param)
			case "gt", "gtfield":
				param := e.Param()
				if param == "" {
					param = time.Now().Format(time.RFC3339)
				}
				errorMessages[i] = fmt.Sprintf("The field %s is must be greater than %s", e.Field(), param)
			case "datetime":
				errorMessages[i] = fmt.Sprintf("The field %s is must have the following date time format: %s", e.Field(), e.Param())
			default:
				errorMessages[i] = e.Error()
			}
		}
		return errorMessages
	} else if marshallingErr, ok := err.(*json.UnmarshalTypeError); ok {
		return []string{fmt.Sprintf("The field %s must be a %s", marshallingErr.Field, marshallingErr.Type.String())}
	}
	return []string{err.Error()}
}
```

Which means you get readable errors like  `The field End is must be greater than Start` and `The field Start is must have the following date time format: 2006-01-02T15:04:05Z07:00`

This is just scratching the surface of what you can do with the default gin validators. You can also enhance it with your own custom validators and add their tags to the `parseError` function for a nice comprehensive set of error messages. There are also tons more built in validators that you can read about [here](https://pkg.go.dev/github.com/go-playground/validator#hdr-Baked_In_Validators_and_Tags).

You can find fully working code in my repository here
## Gotchas
### Gins validation engine is a singleton
That means that custom validators are shared across threads (afaik) and I wouldn't depend on 

First off, there's some separate of concerns that will help a lot. At Convictional we've decided to strongly separate HTTP Handlers and Services into separate layers. This approach helps with validation because it allows you to declare dedicated request structs that can hold your validation rules and don't need to match the exact data types and structures of your database. This way you can easily create nullable fields without your service needing to account for that. More on that later though. 
I've found that the more validation I can do in the handler the better. That being said, I've also noticed that validation that's runtime specific (e.g., changes depending on what user is making the request or what data is present in the database) tends to be more naturally at home in the service layer.
### Data Types Matter
Certain validators will only work if the data type is a supported type. For example, `gt` has different effects [depending on the data type](https://pkg.go.dev/github.com/go-playground/validator/v10#hdr-Greater_Than) :
* for `string` it validates that the string is longer than the parameter (e.g., `gt=12` string must be longer than 12 characters)
* for `date` it validates that the date is greater than `time.Now()` and doesn't accept any parameters 
* for a numeric value it will check that the number has a greater value than the parameter
### Pointers for nil values