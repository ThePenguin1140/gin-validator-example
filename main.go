package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"time"

	"github.com/gin-gonic/gin"

	"github.com/go-playground/validator/v10"
)

type params struct {
	Start *string `json:"start" form:"start" binding:"required_without=End,omitempty,datetime=2006-01-02T15:04:05Z07:00"`
	End *string `json:"end" form:"end" binding:"required_without=Start,omitempty,datetime=2006-01-02T15:04:05Z07:00"`
}

type DateRange struct {
	Start *time.Time `form:"start" binding:"omitempty,lt|ltfield=End"`
	End *time.Time `form:"end" binding:"omitempty,gt|gtfield=Start"`
}

func main() {
	r := gin.Default()

	r.GET("/", func(c *gin.Context) {
		params := params{}
		if err := c.ShouldBind(&params); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"errors": parseError(err)})
			return
		}
		now := time.Now()

		dateRange := DateRange{}
		if err := c.ShouldBind(&dateRange); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"errors": parseError(err)})
			return
		}
		if dateRange.End == nil || dateRange.End.IsZero() {
			dateRange.End = &now
		}
		if dateRange.Start == nil || dateRange.Start.IsZero() {
			dateRange.Start = &now
		}
		diff := dateRange.End.Sub(*dateRange.Start)
		c.JSON(http.StatusOK, fmt.Sprintf("%v", diff))
		return
	})
	r.Run() 
}

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