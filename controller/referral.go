package controller

import (
	"errors"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func GetReferralProgramSummary(c *gin.Context) {
	summary, err := model.GetReferralProgramSummary(c.GetInt("id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	common.ApiSuccess(c, summary)
}

func GetReferralUsers(c *gin.Context) {
	level, err := strconv.Atoi(c.Param("level"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	users, err := model.GetReferralUsers(c.GetInt("id"), level)
	if err != nil {
		if errors.Is(err, model.ErrInvalidReferralLevel) {
			common.ApiErrorI18n(c, i18n.MsgInvalidParams)
			return
		}
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	common.ApiSuccess(c, users)
}
