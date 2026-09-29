package captcha_api

import (
	"encoding/base64"
	"errors"

	"github.com/daqing/airway/lib/render"
	"github.com/gin-gonic/gin"

	"github.com/daqing/airway-tencentcloud-plugin/install/lib/captcha"
)

// codeRateLimited matches the send endpoint's body-level code for a rate
// limit, so a client handles both endpoints the same way.
const codeRateLimited = 42901

// ShowAction issues a captcha bound to the requesting IP and returns it as a
// base64 data URL, so the client needs no second request to display it.
func ShowAction(c *gin.Context) {
	token, png, err := captcha.Issue(c.Request.Context(), c.ClientIP())
	if err != nil {
		if errors.Is(err, captcha.ErrRateLimited) {
			render.ErrorCodeMsg(c, codeRateLimited, err.Error())
			return
		}

		render.Error(c, err)
		return
	}

	render.OK(c, gin.H{
		"token": token,
		"image": "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
	})
}
