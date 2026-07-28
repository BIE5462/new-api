package controller

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRecordQuotaAdjustmentAuditForRecordsOperatorAndTarget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalMainDatabaseType := common.MainDatabaseType()
	originalLogDatabaseType := common.LogDatabaseType()
	originalRedisEnabled := common.RedisEnabled
	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.SetDatabaseTypes(originalMainDatabaseType, originalLogDatabaseType)
		common.RedisEnabled = originalRedisEnabled
	})

	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Log{}))

	admin := &model.User{Username: "admin-a", DisplayName: "admin-a", Role: common.RoleAdminUser, AffCode: "admin-a-code"}
	target := &model.User{Username: "user-b", DisplayName: "user-b", Role: common.RoleCommonUser, AffCode: "user-b-code"}
	require.NoError(t, db.Create(admin).Error)
	require.NoError(t, db.Create(target).Error)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/api/user/manage", nil)
	c.Set("id", admin.Id)
	c.Set("username", admin.Username)
	c.Set("role", admin.Role)

	recordQuotaAdjustmentAuditFor(c, target.Id, "user.quota_add", map[string]interface{}{
		"quota": "100,000",
	})

	var logs []model.Log
	require.NoError(t, db.Order("user_id asc").Find(&logs).Error)
	require.Len(t, logs, 2)
	require.Equal(t, admin.Id, logs[0].UserId)
	require.Equal(t, target.Id, logs[1].UserId)
	for _, log := range logs {
		require.Equal(t, model.LogTypeManage, log.Type)
		var other map[string]interface{}
		require.NoError(t, common.Unmarshal([]byte(log.Other), &other))
		op, ok := other["op"].(map[string]interface{})
		require.True(t, ok)
		require.Equal(t, "user.quota_add", op["action"])
		params, ok := op["params"].(map[string]interface{})
		require.True(t, ok)
		require.Equal(t, "100,000", params["quota"])
	}

	userLogs, total, err := model.GetUserLogs(target.Id, 0, 0, 0, "", "", 0, 20, "", "", "")
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, userLogs, 1)
	require.Equal(t, target.Id, userLogs[0].UserId)
}
