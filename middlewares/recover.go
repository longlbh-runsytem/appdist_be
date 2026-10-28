package middlewares

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/longlbh/appdist-be/common"
)

// Recover là lưới an toàn cho panic ngoài dự kiến (nil pointer, index out of
// range...). Luồng lỗi nghiệp vụ đi qua common.Handle, không qua đây.
func Recover() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			r := recover()
			if r == nil {
				return
			}

			err, ok := r.(error)
			if !ok {
				err = fmt.Errorf("%v", r)
			}

			// Ghi log kèm stack trước khi trả lời client.
			log.Error().Err(err).Str("path", c.FullPath()).Msg("panic recovered")

			var appErr *common.AppError
			if !errors.As(err, &appErr) {
				appErr = common.ErrInternal(err)
			}

			// Trả đúng status code, và KHÔNG panic lại: panic tiếp sẽ thoát ra
			// gin.Recovery và ghi đè response vừa viết.
			c.AbortWithStatusJSON(appErr.Code, appErr)
		}()

		c.Next()
	}
}
