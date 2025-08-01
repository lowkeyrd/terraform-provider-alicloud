package alicloud

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PaesslerAG/jsonpath"
	util "github.com/alibabacloud-go/tea-utils/service"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/ram"
	"github.com/aliyun/terraform-provider-alicloud/alicloud/connectivity"
	"github.com/denverdino/aliyungo/common"
	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

type RdsService struct {
	client *connectivity.AliyunClient
}

//	_______________                      _______________                       _______________
//	|              | ______param______\  |              |  _____request_____\  |              |
//	|   Business   |                     |    Service   |                      |    SDK/API   |
//	|              | __________________  |              |  __________________  |              |
//	|______________| \    (obj, err)     |______________|  \ (status, cont)    |______________|
//	                    |                                    |
//	                    |A. {instance, nil}                  |a. {200, content}
//	                    |B. {nil, error}                     |b. {200, nil}
//	               					  |c. {4xx, nil}
//
// The API return 200 for resource not found.
// When getInstance is empty, then throw InstanceNotfound error.
// That the business layer only need to check error.
var DBInstanceStatusCatcher = Catcher{"OperationDenied.DBInstanceStatus", 60, 5}

func (s *RdsService) DescribeDBInstance(id string) (object map[string]interface{}, err error) {
	client := s.client
	action := "DescribeDBInstanceAttribute"
	request := map[string]interface{}{
		"RegionId":     s.client.RegionId,
		"DBInstanceId": id,
		"SourceIp":     s.client.SourceIp,
	}
	var response map[string]interface{}
	wait := incrementalWait(3*time.Second, 3*time.Second)
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		response, err = client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
		if err != nil {
			if NeedRetry(err) || IsExpectedErrors(err, []string{"InvalidParameter"}) {
				wait()
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		addDebug(action, response, request)
		return nil
	})
	if err != nil {
		if IsExpectedErrors(err, []string{"InvalidDBInstanceId.NotFound", "InvalidDBInstanceName.NotFound"}) {
			return nil, WrapErrorf(err, NotFoundMsg, AlibabaCloudSdkGoERROR)
		}
		return nil, WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
	}
	v, err := jsonpath.Get("$.Items.DBInstanceAttribute", response)
	if err != nil {
		return nil, WrapErrorf(err, FailedGetAttributeMsg, id, "$.Items.DBInstanceAttribute", response)
	}
	if len(v.([]interface{})) < 1 {
		return nil, WrapErrorf(NotFoundErr("DBAccount", id), NotFoundMsg, ProviderERROR)
	}
	return v.([]interface{})[0].(map[string]interface{}), nil
}

func (s *RdsService) DescribeTasks(id string) (object map[string]interface{}, err error) {
	action := "DescribeTasks"
	request := map[string]interface{}{
		"RegionId":     s.client.RegionId,
		"DBInstanceId": id,
		"SourceIp":     s.client.SourceIp,
	}
	client := s.client
	response, err := client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
	if err != nil {
		if IsExpectedErrors(err, []string{"InvalidDBInstanceId.NotFound"}) {
			return nil, WrapErrorf(err, NotFoundMsg, ProviderERROR)
		}
		return nil, WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
	}
	addDebug(action, response, request)
	return response, nil
}

func (s *RdsService) DescribeDBReadonlyInstance(id string) (object map[string]interface{}, err error) {
	action := "DescribeDBInstanceAttribute"
	request := map[string]interface{}{
		"RegionId":     s.client.RegionId,
		"DBInstanceId": id,
		"SourceIp":     s.client.SourceIp,
	}
	client := s.client
	response, err := client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
	if err != nil {
		if IsExpectedErrors(err, []string{"InvalidDBInstanceId.NotFound"}) {
			return nil, WrapErrorf(err, NotFoundMsg, AlibabaCloudSdkGoERROR)
		}
		return nil, WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
	}
	addDebug(action, response, request)
	dBInstanceAttributes := response["Items"].(map[string]interface{})["DBInstanceAttribute"].([]interface{})
	if len(dBInstanceAttributes) < 1 {
		return nil, WrapErrorf(NotFoundErr("DBInstance", id), NotFoundMsg, ProviderERROR)
	}

	return dBInstanceAttributes[0].(map[string]interface{}), nil
}

func (s *RdsService) DescribeDBAccountPrivilege(id string) (object map[string]interface{}, err error) {
	var ds map[string]interface{}
	parts, err := ParseResourceId(id, 3)
	if err != nil {
		return ds, WrapError(err)
	}
	action := "DescribeAccounts"
	request := map[string]interface{}{
		"RegionId":     s.client.RegionId,
		"DBInstanceId": parts[0],
		"AccountName":  parts[1],
		"SourceIp":     s.client.SourceIp,
	}
	client := s.client
	invoker := NewInvoker()
	invoker.AddCatcher(DBInstanceStatusCatcher)
	var response map[string]interface{}
	if err := invoker.Run(func() error {
		response, err = client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
		if err != nil {
			return WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
		}
		addDebug(action, response, request)
		return nil
	}); err != nil {
		if IsExpectedErrors(err, []string{"InvalidDBInstanceId.NotFound"}) {
			return ds, WrapErrorf(err, NotFoundMsg, AlibabaCloudSdkGoERROR)
		}
		return ds, WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
	}
	dBInstanceAccounts := response["Accounts"].(map[string]interface{})["DBInstanceAccount"].([]interface{})
	if len(dBInstanceAccounts) < 1 {
		return ds, WrapErrorf(NotFoundErr("DBAccountPrivilege", id), NotFoundMsg, ProviderERROR)
	}
	return dBInstanceAccounts[0].(map[string]interface{}), nil
}

func (s *RdsService) DescribeDBDatabase(id string) (object map[string]interface{}, err error) {
	var ds map[string]interface{}
	parts, err := ParseResourceId(id, 2)
	if err != nil {
		return ds, WrapError(err)
	}
	dbName := parts[1]
	var response map[string]interface{}
	action := "DescribeDatabases"
	request := map[string]interface{}{
		"RegionId":     s.client.RegionId,
		"DBInstanceId": parts[0],
		"DBName":       dbName,
		"SourceIp":     s.client.SourceIp,
	}
	client := s.client
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		response, err = client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
		if err != nil {
			if IsExpectedErrors(err, []string{"InternalError", "OperationDenied.DBInstanceStatus"}) {
				return resource.RetryableError(WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR))
			}
			if NotFoundError(err) || IsExpectedErrors(err, []string{"InvalidDBName.NotFound", "InvalidDBInstanceId.NotFoundError"}) {
				return resource.NonRetryableError(WrapErrorf(err, NotFoundMsg, AlibabaCloudSdkGoERROR))
			}
			return resource.NonRetryableError(WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR))
		}
		addDebug(action, response, request)
		v, err := jsonpath.Get("$.Databases.Database", response)
		if err != nil {
			return resource.NonRetryableError(WrapErrorf(err, FailedGetAttributeMsg, id, "$.Databases.Database", response))
		}
		if len(v.([]interface{})) < 1 {
			return resource.NonRetryableError(WrapErrorf(NotFoundErr("DBDatabase", dbName), NotFoundMsg, ProviderERROR))
		}
		ds = v.([]interface{})[0].(map[string]interface{})
		return nil
	})
	return ds, err
}

func (s *RdsService) DescribeParameters(id string) (object map[string]interface{}, err error) {
	client := s.client
	action := "DescribeParameters"
	request := map[string]interface{}{
		"RegionId":     s.client.RegionId,
		"DBInstanceId": id,
		"SourceIp":     s.client.SourceIp,
	}
	runtime := util.RuntimeOptions{}
	runtime.SetAutoretry(true)
	response, err := client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
	if err != nil {
		if IsExpectedErrors(err, []string{"InvalidDBInstanceId.NotFound"}) {
			return nil, WrapErrorf(err, NotFoundMsg, AlibabaCloudSdkGoERROR)
		}
		return nil, WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
	}
	addDebug(action, response, request)
	return response, err
}

func (s *RdsService) DescribeInstanceLinkedWhitelistTemplate(id string) (object map[string]interface{}, err error) {
	client := s.client
	action := "DescribeInstanceLinkedWhitelistTemplate"
	request := map[string]interface{}{
		"RegionId": s.client.RegionId,
		"InsName":  id,
	}
	response, err := client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
	if err != nil {
		if IsExpectedErrors(err, []string{}) {
			return nil, WrapErrorf(err, NotFoundMsg, AlibabaCloudSdkGoERROR)
		}
		return nil, WrapErrorf(err, DefaultErrorMsg, action, AlibabaCloudSdkGoERROR)
	}
	addDebug(action, response, request)

	object = make(map[string]interface{})

	v, err := jsonpath.Get("$.Data.Templates", response)
	if err != nil {
		return object, WrapErrorf(err, FailedGetAttributeMsg, action, "$.Data.Templates", response)
	}

	templates, ok := v.([]interface{})
	if !ok {
		return nil, WrapErrorf(Error("Failed to parse Templates as array"), FailedGetAttributeMsg, action, "$.Data.Templates", response)
	}

	var templateIds []int
	for _, item := range templates {
		templateMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		if templateId, ok := templateMap["TemplateId"].(json.Number); ok {
			if id, err := strconv.Atoi(templateId.String()); err == nil {
				templateIds = append(templateIds, id)
			} else {
				log.Printf("[WARN] Failed to convert json.Number TemplateId to int: %s", templateId.String())
			}
		} else if templateIdStr, ok := templateMap["TemplateId"].(string); ok {
			if id, err := strconv.Atoi(templateIdStr); err == nil {
				templateIds = append(templateIds, id)
			} else {
				log.Printf("[WARN] Failed to convert string TemplateId to int: %s", templateIdStr)
			}
		} else {
			log.Printf("[WARN] Invalid TemplateId type in response: %T", templateMap["TemplateId"])
		}
	}
	log.Printf("[DEBUG] Generated TemplateIds: %v", templateIds)

	object["Templates"] = templates
	object["TemplateIds"] = templateIds

	return object, nil
}

func (s *RdsService) SetTimeZone(d *schema.ResourceData) error {
	targetParameterName := ""
	engine := d.Get("engine")
	if engine == string(MySQL) {
		targetParameterName = "default_time_zone"
	} else if engine == string(PostgreSQL) {
		targetParameterName = "timezone"
	}

	if targetParameterName != "" {
		paramsRes, err := s.DescribeParameters(d.Id())
		if err != nil {
			return WrapError(err)
		}
		parameters := paramsRes["RunningParameters"].(map[string]interface{})["DBInstanceParameter"].([]interface{})
		for _, item := range parameters {
			item := item.(map[string]interface{})
			parameterName := item["ParameterName"]

			if parameterName == targetParameterName {
				d.Set("db_time_zone", item["ParameterValue"])
				break
			}
		}
	}
	return nil
}

func (s *RdsService) RefreshParameters(d *schema.ResourceData, attribute string) error {
	var param []map[string]interface{}
	documented, ok := d.GetOk(attribute)
	if !ok {
		return nil
	}
	object, err := s.DescribeParameters(d.Id())
	if err != nil {
		return WrapError(err)
	}

	var parameters = make(map[string]interface{})
	dBInstanceParameters := object["RunningParameters"].(map[string]interface{})["DBInstanceParameter"].([]interface{})
	for _, i := range dBInstanceParameters {
		i := i.(map[string]interface{})
		if i["ParameterName"] != "" {
			parameter := map[string]interface{}{
				"name":  i["ParameterName"],
				"value": i["ParameterValue"],
			}
			parameters[i["ParameterName"].(string)] = parameter
		}
	}
	dBInstanceParameters = object["ConfigParameters"].(map[string]interface{})["DBInstanceParameter"].([]interface{})
	for _, i := range dBInstanceParameters {
		i := i.(map[string]interface{})
		if i["ParameterName"] != "" {
			parameter := map[string]interface{}{
				"name":  i["ParameterName"],
				"value": i["ParameterValue"],
			}
			parameters[i["ParameterName"].(string)] = parameter
		}
	}

	for _, parameter := range documented.(*schema.Set).List() {
		name := parameter.(map[string]interface{})["name"]
		for _, value := range parameters {
			if value.(map[string]interface{})["name"] == name {
				param = append(param, value.(map[string]interface{}))
				break
			}
		}
	}
	if len(param) > 0 {
		if err := d.Set(attribute, param); err != nil {
			return WrapError(err)
		}
	}
	return nil
}

func (s *RdsService) RefreshPgHbaConf(d *schema.ResourceData, attribute string) error {
	response, err := s.DescribePGHbaConfig(d.Id())
	if err != nil {
		return WrapError(err)
	}
	runningHbaItems := make([]interface{}, 0)
	if v, exist := response["RunningHbaItems"].(map[string]interface{})["HbaItem"]; exist {
		runningHbaItems = v.([]interface{})
	}

	var items []map[string]interface{}

	documented, ok := d.GetOk(attribute)
	if !ok {
		return nil
	}

	for _, item := range documented.(*schema.Set).List() {
		item := item.(map[string]interface{})
		for _, item2 := range runningHbaItems {
			item2 := item2.(map[string]interface{})
			if item["priority_id"] == formatInt(item2["PriorityId"]) {
				mapping := map[string]interface{}{
					"type":        item2["Type"],
					"database":    item2["Database"],
					"priority_id": formatInt(item2["PriorityId"]),
					"address":     item2["Address"],
					"user":        item2["User"],
					"method":      item2["Method"],
					"option":      item2["Option"],
					"mask":        item2["Mask"],
				}
				if item2["mask"] != nil && item2["mask"] != "" {
					mapping["mask"] = item2["mask"]
				}
				if item2["option"] != nil && item2["option"] != "" {
					mapping["option"] = item2["option"]
				}
				items = append(items, mapping)
			}
		}
	}
	if len(items) > 0 {
		if err := d.Set(attribute, items); err != nil {
			return WrapError(err)
		}
	}
	return nil
}

func (s *RdsService) ModifyPgHbaConfig(d *schema.ResourceData, attribute string) error {
	client := s.client
	var err error
	action := "ModifyPGHbaConfig"
	request := map[string]interface{}{
		"RegionId":     s.client.RegionId,
		"DBInstanceId": d.Id(),
		"SourceIp":     s.client.SourceIp,
	}
	request["OpsType"] = "update"
	pgHbaConfig := d.Get("pg_hba_conf")
	count := 1
	for _, i := range pgHbaConfig.(*schema.Set).List() {
		i := i.(map[string]interface{})
		request[fmt.Sprint("HbaItem.", count, ".Type")] = i["type"]
		if i["mask"] != nil && i["mask"] != "" {
			request[fmt.Sprint("HbaItem.", count, ".Mask")] = i["mask"]
		}
		request[fmt.Sprint("HbaItem.", count, ".Database")] = i["database"]
		request[fmt.Sprint("HbaItem.", count, ".PriorityId")] = i["priority_id"]
		request[fmt.Sprint("HbaItem.", count, ".Address")] = i["address"]
		request[fmt.Sprint("HbaItem.", count, ".User")] = i["user"]
		request[fmt.Sprint("HbaItem.", count, ".Method")] = i["method"]
		if i["option"] != nil && i["mask"] != "" {
			request[fmt.Sprint("HbaItem.", count, ".Option")] = i["option"]
		}
		count = count + 1
	}
	var response map[string]interface{}
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		response, err = client.RpcPost("Rds", "2014-08-15", action, nil, request, false)
		if err != nil {
			if IsExpectedErrors(err, []string{"InternalError"}) {
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		addDebug(action, response, request)
		return nil
	})
	if err != nil {
		return WrapError(err)
	}
	if err := s.WaitForDBInstance(d.Id(), Running, DefaultLongTimeout); err != nil {
		return WrapError(err)
	}

	desResponse, err := s.DescribePGHbaConfig(d.Id())
	if err != nil {
		return WrapError(err)
	}
	if desResponse["LastModifyStatus"] == "failed" {
		return WrapError(Error("%v", desResponse["ModifyStatusReason"].(string)))
	}
	d.SetPartial(attribute)
	return nil
}

func (s *RdsService) ModifyDBInstanceDeletionProtection(d *schema.ResourceData, attribute string) error {
	client := s.client
	var err error
	action := "ModifyDBInstanceDeletionProtection"
	request := map[string]interface{}{
		"RegionId":     s.client.RegionId,
		"DBInstanceId": d.Id(),
		"SourceIp":     s.client.SourceIp,
	}
	request["DeletionProtection"] = d.Get("deletion_protection")
	var response map[string]interface{}
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		response, err = client.RpcPost("Rds", "2014-08-15", action, nil, request, false)
		if err != nil {
			if IsExpectedErrors(err, []string{"InternalError"}) || NeedRetry(err) {
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		addDebug(action, response, request)
		return nil
	})
	if err != nil {
		return WrapError(err)
	}
	if err := s.WaitForDBInstance(d.Id(), Running, DefaultLongTimeout); err != nil {
		return WrapError(err)
	}
	d.SetPartial(attribute)
	return nil
}

func (s *RdsService) ModifyHADiagnoseConfig(d *schema.ResourceData, attribute string) error {
	client := s.client
	action := "ModifyHADiagnoseConfig"
	request := map[string]interface{}{
		"RegionId":     s.client.RegionId,
		"DBInstanceId": d.Id(),
		"SourceIp":     s.client.SourceIp,
	}
	request["TcpConnectionType"] = d.Get("tcp_connection_type")
	var response map[string]interface{}
	var err error
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		response, err = client.RpcPost("Rds", "2014-08-15", action, nil, request, false)
		if err != nil {
			if IsExpectedErrors(err, []string{"InternalError"}) || NeedRetry(err) {
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		addDebug(action, response, request)
		return nil
	})
	if err != nil {
		return WrapError(err)
	}
	if err := s.WaitForDBInstance(d.Id(), Running, DefaultLongTimeout); err != nil {
		return WrapError(err)
	}
	d.SetPartial(attribute)
	return nil
}

func (s *RdsService) ModifyParameters(d *schema.ResourceData, attribute string) error {
	client := s.client
	action := "ModifyParameter"
	request := map[string]interface{}{
		"RegionId":     s.client.RegionId,
		"DBInstanceId": d.Id(),
		"Forcerestart": d.Get("force_restart"),
		"SourceIp":     s.client.SourceIp,
	}
	config := make(map[string]string)
	allConfig := make(map[string]string)
	o, n := d.GetChange(attribute)
	os, ns := o.(*schema.Set), n.(*schema.Set)
	add := ns.Difference(os).List()
	if len(add) > 0 {
		for _, i := range add {
			key := i.(map[string]interface{})["name"].(string)
			value := i.(map[string]interface{})["value"].(string)
			config[key] = value
		}
		cfg, _ := json.Marshal(config)
		request["Parameters"] = string(cfg)
		// wait instance status is Normal before modifying
		if err := s.WaitForDBInstance(d.Id(), Running, DefaultLongTimeout); err != nil {
			return WrapError(err)
		}
		// Need to check whether some parameter needs restart
		if !d.Get("force_restart").(bool) {
			action := "DescribeParameterTemplates"
			request := map[string]interface{}{
				"RegionId":      s.client.RegionId,
				"DBInstanceId":  d.Id(),
				"Engine":        d.Get("engine"),
				"EngineVersion": d.Get("engine_version"),
				"ClientToken":   buildClientToken(action),
				"SourceIp":      s.client.SourceIp,
			}
			forceRestartMap := make(map[string]string)
			response, err := client.RpcPost("Rds", "2014-08-15", action, nil, request, false)
			if err != nil {
				return WrapErrorf(err, DefaultErrorMsg, d.Id(), action, AlibabaCloudSdkGoERROR)
			}
			addDebug(action, response, request)
			stateConf := BuildStateConf([]string{}, []string{"Running"}, d.Timeout(schema.TimeoutUpdate), 3*time.Minute, s.RdsDBInstanceStateRefreshFunc(d.Id(), []string{"Deleting"}))
			if _, err := stateConf.WaitForState(); err != nil {
				return WrapErrorf(err, IdMsg, d.Id())
			}
			templateRecords := response["Parameters"].(map[string]interface{})["TemplateRecord"].([]interface{})
			for _, para := range templateRecords {
				para := para.(map[string]interface{})
				if para["ForceRestart"] == "true" {
					forceRestartMap[para["ParameterName"].(string)] = para["ForceRestart"].(string)
				}
			}
			if len(forceRestartMap) > 0 {
				for key := range config {
					if _, ok := forceRestartMap[key]; ok {
						return WrapError(fmt.Errorf("Modifying RDS instance's parameter '%s' requires setting 'force_restart = true'.", key))
					}
				}
			}
		}
		response, err := client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
		if err != nil {
			return WrapErrorf(err, DefaultErrorMsg, d.Id(), action, AlibabaCloudSdkGoERROR)
		}
		addDebug(action, response, request)
		// wait instance parameter expect after modifying
		for _, i := range ns.List() {
			key := i.(map[string]interface{})["name"].(string)
			value := i.(map[string]interface{})["value"].(string)
			allConfig[key] = value
		}
		if err := s.WaitForDBParameter(d.Id(), DefaultLongTimeout, allConfig); err != nil {
			return WrapError(err)
		}
		// wait instance status is Normal after modifying
		if err := s.WaitForDBInstance(d.Id(), Running, DefaultLongTimeout); err != nil {
			return WrapError(err)
		}
	}
	d.SetPartial(attribute)
	return nil
}

func (s *RdsService) DescribeDBInstanceRwNetInfoByMssql(id string) ([]interface{}, error) {
	action := "DescribeDBInstanceNetInfo"
	request := map[string]interface{}{
		"RegionId":                 s.client.RegionId,
		"DBInstanceId":             id,
		"SourceIp":                 s.client.SourceIp,
		"DBInstanceNetRWSplitType": "ReadWriteSplitting",
	}
	client := s.client
	var response map[string]interface{}
	var err error
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		response, err = client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
		if err != nil {
			if IsExpectedErrors(err, []string{"InternalError"}) {
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		addDebug(action, response, request)
		return nil
	})
	if err != nil {
		if err != nil {
			if IsExpectedErrors(err, []string{"InvalidDBInstanceId.NotFound"}) {
				return nil, WrapErrorf(err, NotFoundMsg, AlibabaCloudSdkGoERROR)
			}
			return nil, WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
		}
	}
	return response["DBInstanceNetInfos"].(map[string]interface{})["DBInstanceNetInfo"].([]interface{}), nil
}

func (s *RdsService) DescribeDBInstanceNetInfo(id string) ([]interface{}, error) {
	action := "DescribeDBInstanceNetInfo"
	request := map[string]interface{}{
		"RegionId":     s.client.RegionId,
		"DBInstanceId": id,
		"SourceIp":     s.client.SourceIp,
	}
	client := s.client
	var response map[string]interface{}
	var err error
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		response, err = client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
		if err != nil {
			if IsExpectedErrors(err, []string{"InternalError"}) {
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		addDebug(action, response, request)
		return nil
	})
	if err != nil {
		if err != nil {
			if IsExpectedErrors(err, []string{"InvalidDBInstanceId.NotFound"}) {
				return nil, WrapErrorf(err, NotFoundMsg, AlibabaCloudSdkGoERROR)
			}
			return nil, WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
		}
	}
	return response["DBInstanceNetInfos"].(map[string]interface{})["DBInstanceNetInfo"].([]interface{}), nil
}

func (s *RdsService) DescribeDBConnection(id string) (map[string]interface{}, error) {
	parts, err := ParseResourceId(id, 2)
	if err != nil {
		return nil, WrapError(err)
	}
	object, err := s.DescribeDBInstanceNetInfo(parts[0])

	if err != nil {
		if IsExpectedErrors(err, []string{"InvalidCurrentConnectionString.NotFound"}) {
			return nil, WrapErrorf(err, NotFoundMsg, AlibabaCloudSdkGoERROR)
		}
		return nil, WrapError(err)
	}

	if object != nil {
		for _, o := range object {
			o := o.(map[string]interface{})
			if strings.HasPrefix(o["ConnectionString"].(string), parts[1]) {
				return o, nil
			}
		}
	}

	return nil, WrapErrorf(NotFoundErr("DBConnection", id), NotFoundMsg, ProviderERROR)
}
func (s *RdsService) DescribeDBReadWriteSplittingConnection(id string) (map[string]interface{}, error) {
	object, err := s.DescribeDBInstanceRwNetInfoByMssql(id)
	if err != nil && !NotFoundError(err) {
		return nil, err
	}

	if object != nil {
		for _, conn := range object {
			conn := conn.(map[string]interface{})
			if conn["ConnectionStringType"] != "ReadWriteSplitting" {
				continue
			}
			if _, ok := conn["MaxDelayTime"]; ok {
				if conn["MaxDelayTime"] == nil {
					continue
				}
				if _, err := strconv.Atoi(conn["MaxDelayTime"].(string)); err != nil {
					return nil, err
				}
			}
			return conn, nil
		}
	}

	return nil, WrapErrorf(NotFoundErr("ReadWriteSplittingConnection", id), NotFoundMsg, ProviderERROR)
}

func (s *RdsService) GrantAccountPrivilege(id, dbName string) error {
	parts, err := ParseResourceId(id, 3)
	if err != nil {
		return WrapError(err)
	}
	action := "GrantAccountPrivilege"
	request := map[string]interface{}{
		"RegionId":         s.client.RegionId,
		"DBInstanceId":     parts[0],
		"AccountName":      parts[1],
		"DBName":           dbName,
		"AccountPrivilege": parts[2],
		"SourceIp":         s.client.SourceIp,
	}
	var response map[string]interface{}
	client := s.client
	err = resource.Retry(3*time.Minute, func() *resource.RetryError {
		response, err = client.RpcPost("Rds", "2014-08-15", action, nil, request, false)
		if err != nil {
			if IsExpectedErrors(err, OperationDeniedDBStatus) || IsExpectedErrors(err, []string{"InvalidDB.NotFound"}) || NeedRetry(err) {
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		addDebug(action, response, request)
		return nil
	})

	if err != nil {
		return WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
	}
	if err := s.WaitForAccountPrivilege(id, dbName, Available, DefaultTimeoutMedium); err != nil {
		return WrapError(err)
	}

	return nil
}

func (s *RdsService) RevokeAccountPrivilege(id, dbName string) error {
	parts, err := ParseResourceId(id, 3)
	if err != nil {
		return WrapError(err)
	}
	action := "RevokeAccountPrivilege"
	request := map[string]interface{}{
		"RegionId":     s.client.RegionId,
		"DBInstanceId": parts[0],
		"AccountName":  parts[1],
		"DBName":       dbName,
		"SourceIp":     s.client.SourceIp,
	}
	client := s.client
	err = resource.Retry(3*time.Minute, func() *resource.RetryError {
		response, err := client.RpcPost("Rds", "2014-08-15", action, nil, request, false)
		if err != nil {
			if IsExpectedErrors(err, OperationDeniedDBStatus) || NeedRetry(err) {
				return resource.RetryableError(err)
			} else if IsExpectedErrors(err, []string{"InvalidDB.NotFound"}) {
				log.Printf("[WARN] Resource alicloud_db_account_privilege RevokeAccountPrivilege Failed!!! %s", err)
				return nil
			}
			return resource.NonRetryableError(err)
		}
		addDebug(action, response, request)
		return nil
	})

	if err != nil {
		return WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
	}

	if err := s.WaitForAccountPrivilegeRevoked(id, dbName, DefaultTimeoutMedium); err != nil {
		return WrapError(err)
	}

	return nil
}

func (s *RdsService) ReleaseDBPublicConnection(instanceId, connection string) error {
	action := "ReleaseInstancePublicConnection"
	request := map[string]interface{}{
		"RegionId":                s.client.RegionId,
		"DBInstanceId":            instanceId,
		"CurrentConnectionString": connection,
		"SourceIp":                s.client.SourceIp,
	}
	client := s.client
	response, err := client.RpcPost("Rds", "2014-08-15", action, nil, request, false)
	if err != nil {
		return WrapErrorf(err, DefaultErrorMsg, instanceId, action, AlibabaCloudSdkGoERROR)
	}
	addDebug(action, response, request)
	return nil
}

func (s *RdsService) ModifyDBBackupPolicy(d *schema.ResourceData, updateForData, updateForLog bool) error {
	enableBackupLog := "1"
	backupPeriod := ""
	if v, ok := d.GetOk("preferred_backup_period"); ok && v.(*schema.Set).Len() > 0 {
		periodList := expandStringList(v.(*schema.Set).List())
		backupPeriod = fmt.Sprintf("%s", strings.Join(periodList[:], COMMA_SEPARATED))
	} else {
		periodList := expandStringList(d.Get("backup_period").(*schema.Set).List())
		backupPeriod = fmt.Sprintf("%s", strings.Join(periodList[:], COMMA_SEPARATED))
	}

	backupTime := "02:00Z-03:00Z"
	if v, ok := d.GetOk("preferred_backup_time"); ok && v.(string) != "02:00Z-03:00Z" {
		backupTime = v.(string)
	} else if v, ok := d.GetOk("backup_time"); ok && v.(string) != "" {
		backupTime = v.(string)
	}

	retentionPeriod := "7"
	if v, ok := d.GetOk("backup_retention_period"); ok && v.(int) != 7 {
		retentionPeriod = strconv.Itoa(v.(int))
	} else if v, ok := d.GetOk("retention_period"); ok && v.(int) != 0 {
		retentionPeriod = strconv.Itoa(v.(int))
	}

	logBackupRetentionPeriod := ""
	if v, ok := d.GetOk("log_backup_retention_period"); ok && v.(int) != 0 {
		logBackupRetentionPeriod = strconv.Itoa(v.(int))
	} else if v, ok := d.GetOk("log_retention_period"); ok && v.(int) != 0 {
		logBackupRetentionPeriod = strconv.Itoa(v.(int))
	}

	localLogRetentionHours := ""
	if v, ok := d.GetOk("local_log_retention_hours"); ok {
		localLogRetentionHours = strconv.Itoa(v.(int))
	}

	localLogRetentionSpace := ""
	if v, ok := d.GetOk("local_log_retention_space"); ok {
		localLogRetentionSpace = strconv.Itoa(v.(int))
	}

	highSpaceUsageProtection := d.Get("high_space_usage_protection").(string)

	if !d.Get("enable_backup_log").(bool) {
		enableBackupLog = "0"
	}

	if d.HasChange("log_backup_retention_period") {
		if d.Get("log_backup_retention_period").(int) > d.Get("backup_retention_period").(int) {
			logBackupRetentionPeriod = retentionPeriod
		}
	}

	logBackupFrequency := ""
	if v, ok := d.GetOk("log_backup_frequency"); ok {
		logBackupFrequency = v.(string)
	}

	compressType := ""
	if v, ok := d.GetOk("compress_type"); ok {
		compressType = v.(string)
	}

	archiveBackupRetentionPeriod := "0"
	if v, ok := d.GetOk("archive_backup_retention_period"); ok {
		archiveBackupRetentionPeriod = strconv.Itoa(v.(int))
	}

	archiveBackupKeepCount := "1"
	if v, ok := d.GetOk("archive_backup_keep_count"); ok {
		archiveBackupKeepCount = strconv.Itoa(v.(int))
	}

	archiveBackupKeepPolicy := "0"
	if v, ok := d.GetOk("archive_backup_keep_policy"); ok {
		archiveBackupKeepPolicy = v.(string)
	}

	releasedKeepPolicy := ""
	if v, ok := d.GetOk("released_keep_policy"); ok {
		releasedKeepPolicy = v.(string)
	}

	category := ""
	if v, ok := d.GetOk("category"); ok {
		category = v.(string)
	}
	backupInterval := "-1"
	if v, ok := d.GetOk("backup_interval"); ok {
		backupInterval = v.(string)
	}
	enableIncrementDataBackup := false
	if v, ok := d.GetOkExists("enable_increment_data_backup"); ok {
		enableIncrementDataBackup = v.(bool)
	}
	backupMethod := "Physical"
	if v, ok := d.GetOk("backup_method"); ok {
		backupMethod = v.(string)
	}
	logBackupLocalRetentionNumber := 60
	if v, ok := d.GetOk("log_backup_local_retention_number"); ok {
		logBackupLocalRetentionNumber = v.(int)
	}
	runtime := util.RuntimeOptions{}
	runtime.SetAutoretry(true)
	instance, err := s.DescribeDBInstance(d.Id())
	if err != nil {
		return WrapError(err)
	}
	if updateForData {
		client := s.client
		action := "ModifyBackupPolicy"
		request := map[string]interface{}{
			"RegionId":              s.client.RegionId,
			"DBInstanceId":          d.Id(),
			"PreferredBackupPeriod": backupPeriod,
			"PreferredBackupTime":   backupTime,
			"BackupRetentionPeriod": retentionPeriod,
			"CompressType":          compressType,
			"BackupPolicyMode":      "DataBackupPolicy",
			"SourceIp":              s.client.SourceIp,
			"ReleasedKeepPolicy":    releasedKeepPolicy,
			"Category":              category,
		}
		if instance["Engine"] == "SQLServer" && instance["Category"] == "AlwaysOn" {
			if v, ok := d.GetOk("backup_priority"); ok {
				request["BackupPriority"] = v.(int)
			}

		}
		if instance["Engine"] == "SQLServer" && instance["DBInstanceStorageType"] != "local_ssd" {
			request["EnableIncrementDataBackup"] = enableIncrementDataBackup
			request["BackupMethod"] = backupMethod
		}
		if instance["Engine"] == "SQLServer" && logBackupFrequency == "LogInterval" {
			request["LogBackupFrequency"] = logBackupFrequency
		}
		if instance["Engine"] == "MySQL" && instance["DBInstanceStorageType"] == "local_ssd" {
			request["ArchiveBackupRetentionPeriod"] = archiveBackupRetentionPeriod
			request["ArchiveBackupKeepCount"] = archiveBackupKeepCount
			request["ArchiveBackupKeepPolicy"] = archiveBackupKeepPolicy
		}
		if (instance["Engine"] == "MySQL" || instance["Engine"] == "PostgreSQL") && instance["DBInstanceStorageType"] != "local_ssd" {
			request["BackupInterval"] = backupInterval
		}

		response, err := client.RpcPost("Rds", "2014-08-15", action, nil, request, false)
		if err != nil {
			return WrapErrorf(err, DefaultErrorMsg, d.Id(), action, AlibabaCloudSdkGoERROR)
		}
		addDebug(action, response, request)
		if err := s.WaitForDBInstance(d.Id(), Running, DefaultTimeoutMedium); err != nil {
			return WrapError(err)
		}
	}

	// At present, the sql server database does not support setting logBackupRetentionPeriod
	if updateForLog && instance["Engine"] != "SQLServer" {
		client := s.client
		action := "ModifyBackupPolicy"
		request := map[string]interface{}{
			"RegionId":                      s.client.RegionId,
			"DBInstanceId":                  d.Id(),
			"EnableBackupLog":               enableBackupLog,
			"LocalLogRetentionHours":        localLogRetentionHours,
			"LocalLogRetentionSpace":        localLogRetentionSpace,
			"HighSpaceUsageProtection":      highSpaceUsageProtection,
			"BackupPolicyMode":              "LogBackupPolicy",
			"LogBackupRetentionPeriod":      logBackupRetentionPeriod,
			"LogBackupLocalRetentionNumber": logBackupLocalRetentionNumber,
			"SourceIp":                      s.client.SourceIp,
		}
		response, err := client.RpcPost("Rds", "2014-08-15", action, nil, request, false)
		if err != nil {
			return WrapErrorf(err, DefaultErrorMsg, d.Id(), action, AlibabaCloudSdkGoERROR)
		}
		addDebug(action, response, request)
		if err := s.WaitForDBInstance(d.Id(), Running, DefaultTimeoutMedium); err != nil {
			return WrapError(err)
		}
	}
	return nil
}

func (s *RdsService) ModifyDBSecurityIps(instanceId, ips string) error {
	client := s.client
	action := "ModifySecurityIps"
	request := map[string]interface{}{
		"RegionId":     s.client.RegionId,
		"DBInstanceId": instanceId,
		"SecurityIps":  ips,
		"SourceIp":     s.client.SourceIp,
	}
	response, err := client.RpcPost("Rds", "2014-08-15", action, nil, request, false)
	if err != nil {
		return WrapErrorf(err, DefaultErrorMsg, instanceId, action, AlibabaCloudSdkGoERROR)
	}
	addDebug(action, response, request)

	if err := s.WaitForDBInstance(instanceId, Running, DefaultTimeoutMedium); err != nil {
		return WrapError(err)
	}
	return nil
}

func (s *RdsService) DescribeDBSecurityIps(instanceId string) ([]interface{}, error) {
	action := "DescribeDBInstanceIPArrayList"
	request := map[string]interface{}{
		"RegionId":     s.client.RegionId,
		"DBInstanceId": instanceId,
		"SourceIp":     s.client.SourceIp,
	}
	client := s.client
	var response map[string]interface{}
	var err error
	wait := incrementalWait(3*time.Second, 3*time.Second)
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		response, err = client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
		if err != nil {
			if NeedRetry(err) {
				wait()
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		addDebug(action, response, request)
		return nil
	})
	if err != nil {
		return nil, WrapErrorf(err, DefaultErrorMsg, instanceId, action, AlibabaCloudSdkGoERROR)
	}
	return response["Items"].(map[string]interface{})["DBInstanceIPArray"].([]interface{}), nil
}

func (s *RdsService) DescribeParameterTemplates(instanceId, engine, engineVersion string) ([]interface{}, error) {
	action := "DescribeParameterTemplates"
	request := map[string]interface{}{
		"RegionId":      s.client.RegionId,
		"DBInstanceId":  instanceId,
		"Engine":        engine,
		"EngineVersion": engineVersion,
		"SourceIp":      s.client.SourceIp,
	}
	client := s.client
	response, err := client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
	if err != nil {
		return nil, WrapErrorf(err, DefaultErrorMsg, instanceId, action, AlibabaCloudSdkGoERROR)
	}
	addDebug(action, response, request)
	return response["Parameters"].(map[string]interface{})["TemplateRecord"].([]interface{}), nil
}

func (s *RdsService) GetSecurityIps(instanceId string, dbInstanceIpArrayName string) ([]string, error) {
	object, err := s.DescribeDBSecurityIps(instanceId)
	if err != nil {
		return nil, WrapError(err)
	}

	var ips, separator string
	ipsMap := make(map[string]string)
	for _, ip := range object {
		ip := ip.(map[string]interface{})
		if ip["DBInstanceIPArrayName"].(string) == dbInstanceIpArrayName {
			ips += separator + ip["SecurityIPList"].(string)
			separator = COMMA_SEPARATED
		}
	}

	for _, ip := range strings.Split(ips, COMMA_SEPARATED) {
		ipsMap[ip] = ip
	}

	var finalIps []string
	if len(ipsMap) > 0 {
		for key := range ipsMap {
			finalIps = append(finalIps, key)
		}
	}

	return finalIps, nil
}

func (s *RdsService) DescribeSecurityGroupConfiguration(id string) ([]string, error) {
	action := "DescribeSecurityGroupConfiguration"
	request := map[string]interface{}{
		"RegionId":     s.client.RegionId,
		"DBInstanceId": id,
		"SourceIp":     s.client.SourceIp,
	}
	client := s.client
	var response map[string]interface{}
	var err error
	wait := incrementalWait(3*time.Second, 3*time.Second)
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		response, err = client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
		if err != nil {
			if NeedRetry(err) {
				wait()
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		addDebug(action, response, request)
		return nil
	})
	if err != nil {
		return nil, WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
	}
	groupIds := make([]string, 0)
	ecsSecurityGroupRelations := response["Items"].(map[string]interface{})["EcsSecurityGroupRelation"].([]interface{})
	for _, v := range ecsSecurityGroupRelations {
		v := v.(map[string]interface{})
		groupIds = append(groupIds, v["SecurityGroupId"].(string))
	}
	return groupIds, nil
}

func (s *RdsService) DescribeDBInstanceSSL(id string) (object map[string]interface{}, err error) {
	action := "DescribeDBInstanceSSL"
	request := map[string]interface{}{
		"RegionId":     s.client.RegionId,
		"DBInstanceId": id,
		"SourceIp":     s.client.SourceIp,
	}
	client := s.client
	var response map[string]interface{}
	wait := incrementalWait(3*time.Second, 3*time.Second)
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		response, err = client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
		if err != nil {
			if NeedRetry(err) {
				wait()
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		addDebug(action, response, request)
		return nil
	})
	if err != nil {
		return nil, WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
	}
	return response, nil
}

func (s *RdsService) DescribeDBInstanceEncryptionKey(id string) (object map[string]interface{}, err error) {
	action := "DescribeDBInstanceEncryptionKey"
	request := map[string]interface{}{
		"RegionId":     s.client.RegionId,
		"DBInstanceId": id,
		"SourceIp":     s.client.SourceIp,
	}
	client := s.client
	response, err := client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
	if err != nil {
		return nil, WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
	}
	addDebug(action, response, request)
	return response, nil
}

func (s *RdsService) DescribeHASwitchConfig(id string) (object map[string]interface{}, err error) {
	action := "DescribeHASwitchConfig"
	request := map[string]interface{}{
		"RegionId":     s.client.RegionId,
		"DBInstanceId": id,
		"SourceIp":     s.client.SourceIp,
	}
	client := s.client
	var response map[string]interface{}
	wait := incrementalWait(3*time.Second, 3*time.Second)
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		response, err = client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
		if err != nil {
			if NeedRetry(err) {
				wait()
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		addDebug(action, response, request)
		return nil
	})
	if err != nil {
		return nil, WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
	}
	return response, nil
}

func (s *RdsService) DescribeRdsTDEInfo(id string) (object map[string]interface{}, err error) {
	action := "DescribeDBInstanceTDE"
	request := map[string]interface{}{
		"RegionId":     s.client.RegionId,
		"DBInstanceId": id,
		"SourceIp":     s.client.SourceIp,
	}
	statErr := s.WaitForDBInstance(id, Running, DefaultLongTimeout)
	if statErr != nil {
		return nil, WrapError(statErr)
	}
	client := s.client
	var response map[string]interface{}
	wait := incrementalWait(3*time.Second, 3*time.Second)
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		response, err = client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
		if err != nil {
			if NeedRetry(err) {
				wait()
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		addDebug(action, response, request)
		return nil
	})
	if err != nil {
		return nil, WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
	}
	return response, nil
}

func (s *RdsService) ModifySecurityGroupConfiguration(id string, groupid string) error {
	client := s.client
	action := "ModifySecurityGroupConfiguration"
	request := map[string]interface{}{
		"RegionId":     s.client.RegionId,
		"DBInstanceId": id,
		"SourceIp":     s.client.SourceIp,
	}
	//openapi required that input "Empty" if groupid is ""
	if len(groupid) == 0 {
		groupid = "Empty"
	}
	request["SecurityGroupId"] = groupid
	var response map[string]interface{}
	var err error
	wait := incrementalWait(3*time.Second, 3*time.Second)
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		response, err = client.RpcPost("Rds", "2014-08-15", action, nil, request, false)
		if err != nil {
			if NeedRetry(err) || IsExpectedErrors(err, []string{"ServiceUnavailable"}) {
				wait()
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		addDebug(action, response, request)
		return nil
	})
	if err != nil {
		return WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
	}
	addDebug(action, response, request)
	return nil
}

// return multiIZ list of current region
func (s *RdsService) DescribeMultiIZByRegion() (izs []string, err error) {
	action := "DescribeRegions"
	request := map[string]interface{}{
		"RegionId": s.client.RegionId,
		"SourceIp": s.client.SourceIp,
	}
	client := s.client
	response, err := client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
	if err != nil {
		return nil, WrapErrorf(err, DefaultErrorMsg, "MultiIZByRegion", action, AlibabaCloudSdkGoERROR)
	}
	addDebug(action, response, request)
	regions := response["Regions"].(map[string]interface{})["RDSRegion"].([]interface{})
	zoneIds := []string{}
	for _, r := range regions {
		r := r.(map[string]interface{})
		if r["RegionId"] == string(s.client.Region) && strings.Contains(r["ZoneId"].(string), MULTI_IZ_SYMBOL) {
			zoneIds = append(zoneIds, r["ZoneId"].(string))
		}
	}

	return zoneIds, nil
}

func (s *RdsService) DescribeBackupPolicy(id string) (object map[string]interface{}, err error) {
	action := "DescribeBackupPolicy"
	request := map[string]interface{}{
		"RegionId":     s.client.RegionId,
		"DBInstanceId": id,
		"SourceIp":     s.client.SourceIp,
	}
	client := s.client
	response, err := client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
	if err != nil {
		if IsExpectedErrors(err, []string{"InvalidDBInstanceId.NotFound"}) {
			return nil, WrapErrorf(err, NotFoundMsg, AlibabaCloudSdkGoERROR)
		}
		return nil, WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
	}
	addDebug(action, response, request)
	return response, nil
}

func (s *RdsService) DescribeDbInstanceMonitor(id string) (monitoringPeriod int, err error) {
	action := "DescribeDBInstanceMonitor"
	request := map[string]interface{}{
		"DBInstanceId": id,
		"RegionId":     s.client.RegionId,
		"SourceIp":     s.client.SourceIp,
	}
	client := s.client
	var response map[string]interface{}
	wait := incrementalWait(3*time.Second, 3*time.Second)
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		response, err = client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
		if err != nil {
			if NeedRetry(err) {
				wait()
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		addDebug(action, response, request)
		return nil
	})
	monPeriod, err := strconv.Atoi(response["Period"].(string))
	if err != nil {
		return 0, WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
	}
	return monPeriod, nil
}

func (s *RdsService) DescribeSQLCollectorPolicy(id string) (object map[string]interface{}, err error) {
	action := "DescribeSQLCollectorPolicy"
	request := map[string]interface{}{
		"DBInstanceId": id,
		"RegionId":     s.client.RegionId,
		"SourceIp":     s.client.SourceIp,
	}
	client := s.client
	var response map[string]interface{}
	wait := incrementalWait(3*time.Second, 3*time.Second)
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		response, err = client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
		if err != nil {
			if NeedRetry(err) {
				wait()
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		addDebug(action, response, request)
		return nil
	})
	if err != nil {
		if IsExpectedErrors(err, []string{"InvalidDBInstanceId.NotFound"}) {
			return nil, WrapErrorf(err, NotFoundMsg, AlibabaCloudSdkGoERROR)
		}
		return nil, WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
	}
	return response, nil
}

func (s *RdsService) DescribeSQLCollectorRetention(id string) (object map[string]interface{}, err error) {
	action := "DescribeSQLCollectorRetention"
	request := map[string]interface{}{
		"DBInstanceId": id,
		"RegionId":     s.client.RegionId,
		"SourceIp":     s.client.SourceIp,
	}
	client := s.client
	var response map[string]interface{}
	wait := incrementalWait(3*time.Second, 3*time.Second)
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		response, err = client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
		if err != nil {
			if NeedRetry(err) {
				wait()
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)

		}
		addDebug(action, response, request)
		return nil
	})
	if err != nil {
		if IsExpectedErrors(err, []string{"InvalidDBInstanceId.NotFound"}) {
			return nil, WrapErrorf(err, NotFoundMsg, AlibabaCloudSdkGoERROR)
		}
		return nil, WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
	}
	return response, nil
}

// WaitForInstance waits for instance to given status
func (s *RdsService) WaitForDBInstance(id string, status Status, timeout int) error {
	deadline := time.Now().Add(time.Duration(timeout) * time.Second)
	for {
		object, err := s.DescribeDBInstance(id)
		if err != nil {
			if NotFoundError(err) {
				if status == Deleted {
					return nil
				}
			} else {
				return WrapError(err)
			}
		}
		if object != nil && strings.ToLower(fmt.Sprint(object["DBInstanceStatus"])) == strings.ToLower(string(status)) {
			break
		}
		time.Sleep(DefaultIntervalShort * time.Second)
		if time.Now().After(deadline) {
			return WrapErrorf(err, WaitTimeoutMsg, id, GetFunc(1), timeout, object["DBInstanceStatus"], status, ProviderERROR)
		}
	}
	return nil
}

func (s *RdsService) RdsDBInstanceStateRefreshFunc(id string, failStates []string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		object, err := s.DescribeDBInstance(id)
		if err != nil {
			if NotFoundError(err) {
				// Set this to nil as if we didn't find anything.
				return nil, "", nil
			}
			return nil, "", WrapError(err)
		}

		for _, failState := range failStates {
			if object["DBInstanceStatus"] == failState {
				return object, fmt.Sprint(object["DBInstanceStatus"]), WrapError(Error(FailedToReachTargetStatus, object["DBInstanceStatus"]))
			}
		}
		return object, fmt.Sprint(object["DBInstanceStatus"]), nil
	}
}
func (s *RdsService) RdsDBInstanceNodeIdRefreshFunc(id string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		describeDBInstanceHAConfigObject, err := s.DescribeDBInstanceHAConfig(id)
		if err != nil {
			if NotFoundError(err) {
				// Set this to nil as if we didn't find anything.
				return nil, "", nil
			}
			return nil, "", WrapError(err)
		}
		hostInstanceInfos := describeDBInstanceHAConfigObject["HostInstanceInfos"].(map[string]interface{})["NodeInfo"].([]interface{})
		var nodeId string
		for _, val := range hostInstanceInfos {
			item := val.(map[string]interface{})
			nodeType := item["NodeType"].(string)
			if nodeType == "Master" {
				nodeId = item["NodeId"].(string)
				break // 停止遍历
			}
		}
		return describeDBInstanceHAConfigObject, fmt.Sprint(nodeId), nil
	}

}
func (s *RdsService) RdsTaskStateRefreshFunc(id string, taskAction string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		object, err := s.DescribeTasks(id)
		if err != nil {
			if NotFoundError(err) {
				// Set this to nil as if we didn't find anything.
				return nil, "", nil
			}
			return nil, "", WrapError(err)
		}
		taskProgressInfos := object["Items"].(map[string]interface{})["TaskProgressInfo"].([]interface{})
		for _, t := range taskProgressInfos {
			t := t.(map[string]interface{})
			if t["TaskAction"] == taskAction {
				return object, t["Status"].(string), nil
			}
		}

		return object, "Pending", nil
	}
}

// WaitForDBParameter waits for instance parameter to given value.
// Status of DB instance is Running after ModifyParameters API was
// call, so we can not just wait for instance status become
// Running, we should wait until parameters have expected values.
func (s *RdsService) WaitForDBParameter(instanceId string, timeout int, expects map[string]string) error {
	deadline := time.Now().Add(time.Duration(timeout) * time.Second)
	for {
		object, err := s.DescribeParameters(instanceId)
		if err != nil {
			return WrapError(err)
		}
		var actuals = make(map[string]string)
		dBInstanceParameters := object["RunningParameters"].(map[string]interface{})["DBInstanceParameter"].([]interface{})
		for _, i := range dBInstanceParameters {
			i := i.(map[string]interface{})
			if i["ParameterName"] == nil || i["ParameterValue"] == nil {
				continue
			}
			actuals[i["ParameterName"].(string)] = i["ParameterValue"].(string)
		}
		dBInstanceParameters = object["ConfigParameters"].(map[string]interface{})["DBInstanceParameter"].([]interface{})
		for _, i := range dBInstanceParameters {
			i := i.(map[string]interface{})
			if i["ParameterName"] == nil || i["ParameterValue"] == nil {
				continue
			}
			actuals[i["ParameterName"].(string)] = i["ParameterValue"].(string)
		}

		match := true

		got_value := ""
		expected_value := ""

		for name, expect := range expects {
			if actual, ok := actuals[name]; ok {
				if expect != actual {
					match = false
					got_value = actual
					expected_value = expect
					break
				}
			} else {
				match = false
			}
		}

		if match {
			break
		}

		time.Sleep(DefaultIntervalShort * time.Second)

		if time.Now().After(deadline) {
			return WrapErrorf(err, WaitTimeoutMsg, instanceId, GetFunc(1), timeout, got_value, expected_value, ProviderERROR)
		}
	}
	return nil
}

func (s *RdsService) WaitForDBConnection(id string, status Status, timeout int) error {
	deadline := time.Now().Add(time.Duration(timeout) * time.Second)
	for {
		object, err := s.DescribeDBConnection(id)
		if err != nil {
			if NotFoundError(err) {
				if status == Deleted {
					return nil
				}
			} else {
				return WrapError(err)
			}
		}
		if object != nil && object["ConnectionString"] != "" {
			return nil
		}
		if time.Now().After(deadline) {
			return WrapErrorf(err, WaitTimeoutMsg, id, GetFunc(1), timeout, object["ConnectionString"], id, ProviderERROR)
		}
	}
}

func (s *RdsService) WaitForDBReadWriteSplitting(id string, status Status, timeout int) error {
	deadline := time.Now().Add(time.Duration(timeout) * time.Second)
	for {
		object, err := s.DescribeDBReadWriteSplittingConnection(id)
		if err != nil {
			if NotFoundError(err) {
				if status == Deleted {
					return nil
				}
			} else {
				return WrapError(err)
			}
		}
		if err == nil {
			break
		}
		time.Sleep(DefaultIntervalShort * time.Second)
		if time.Now().After(deadline) {
			return WrapErrorf(err, WaitTimeoutMsg, id, GetFunc(1), timeout, object["ConnectionString"], id, ProviderERROR)
		}
	}
	return nil
}

func (s *RdsService) WaitForAccountPrivilege(id, dbName string, status Status, timeout int) error {
	parts, err := ParseResourceId(id, 3)
	if err != nil {
		return WrapError(err)
	}
	deadline := time.Now().Add(time.Duration(timeout) * time.Second)
	for {
		object, err := s.DescribeDBDatabase(parts[0] + ":" + dbName)
		if err != nil {
			if NotFoundError(err) {
				if status == Deleted {
					return nil
				}
			} else {
				return WrapError(err)
			}
		}
		ready := false
		if object != nil {
			accountPrivilegeInfos := object["Accounts"].(map[string]interface{})["AccountPrivilegeInfo"].([]interface{})
			for _, account := range accountPrivilegeInfos {
				// At present, postgresql response has a bug, DBOwner will be changed to ALL
				// At present, sqlserver response has a bug, DBOwner will be changed to DbOwner
				account := account.(map[string]interface{})
				if account["Account"] == parts[1] && (account["AccountPrivilege"] == parts[2] || (parts[2] == "DBOwner" && (account["AccountPrivilege"] == "ALL" || account["AccountPrivilege"] == "DbOwner"))) {
					ready = true
					break
				}
			}
		}
		if status == Deleted && !ready {
			break
		}
		if ready {
			break
		}
		if time.Now().After(deadline) {
			return WrapErrorf(err, WaitTimeoutMsg, id, GetFunc(1), timeout, "", id, ProviderERROR)
		}
	}
	return nil
}

func (s *RdsService) WaitForAccountPrivilegeRevoked(id, dbName string, timeout int) error {
	parts, err := ParseResourceId(id, 3)
	if err != nil {
		return WrapError(err)
	}
	deadline := time.Now().Add(time.Duration(timeout) * time.Second)
	for {
		object, err := s.DescribeDBDatabase(parts[0] + ":" + dbName)
		if err != nil {
			if NotFoundError(err) {
				return nil
			}
			return WrapError(err)
		}

		exist := false
		if object != nil {
			accountPrivilegeInfo := object["Accounts"].(map[string]interface{})["AccountPrivilegeInfo"].([]interface{})
			for _, account := range accountPrivilegeInfo {
				account := account.(map[string]interface{})
				if account["Account"] == parts[1] && (account["AccountPrivilege"] == parts[2] || (parts[2] == "DBOwner" && account["AccountPrivilege"] == "ALL")) {
					exist = true
					break
				}
			}
		}

		if !exist {
			break
		}
		if time.Now().After(deadline) {
			return WrapErrorf(err, WaitTimeoutMsg, id, GetFunc(1), timeout, "", dbName, ProviderERROR)
		}

	}
	return nil
}

func (s *RdsService) WaitForDBDatabase(id string, status Status, timeout int) error {
	parts, err := ParseResourceId(id, 2)
	if err != nil {
		return WrapError(err)
	}
	deadline := time.Now().Add(time.Duration(timeout) * time.Second)
	for {
		object, err := s.DescribeDBDatabase(id)
		if err != nil {
			if NotFoundError(err) {
				if status == Deleted {
					return nil
				}
			}
			return WrapError(err)
		}
		if object != nil && object["DBName"] == parts[1] {
			break
		}
		time.Sleep(DefaultIntervalShort * time.Second)
		if time.Now().After(deadline) {
			return WrapErrorf(err, WaitTimeoutMsg, id, GetFunc(1), timeout, object["DBName"], parts[1], ProviderERROR)
		}
	}
	return nil
}

// turn period to TimeType
func (s *RdsService) TransformPeriod2Time(period int, chargeType string) (ut int, tt common.TimeType) {
	if chargeType == string(Postpaid) {
		return 1, common.Day
	}

	if period >= 1 && period <= 9 {
		return period, common.Month
	}

	if period == 12 {
		return 1, common.Year
	}

	if period == 24 {
		return 2, common.Year
	}
	return 0, common.Day

}

// turn TimeType to Period
func (s *RdsService) TransformTime2Period(ut int, tt common.TimeType) (period int) {
	if tt == common.Year {
		return 12 * ut
	}

	return ut

}

func (s *RdsService) flattenDBSecurityIPs(list []interface{}) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(list))
	for _, i := range list {
		i := i.(map[string]interface{})
		l := map[string]interface{}{
			"security_ips": i["SecurityIPList"],
		}
		result = append(result, l)
	}
	return result
}

func (s *RdsService) setInstanceTags(d *schema.ResourceData) error {
	if d.HasChange("tags") {
		added, removed := parsingTags(d)
		client := s.client
		var err error
		removedTagKeys := make([]string, 0)
		for _, v := range removed {
			if !ignoredTags(v, "") {
				removedTagKeys = append(removedTagKeys, v)
			}
		}
		if len(removedTagKeys) > 0 {
			action := "UnTagResources"
			request := map[string]interface{}{
				"RegionId":     s.client.RegionId,
				"ResourceType": "INSTANCE",
				"ResourceId.1": d.Id(),
				"SourceIp":     s.client.SourceIp,
			}
			for i, key := range removedTagKeys {
				request[fmt.Sprintf("TagKey.%d", i+1)] = key
			}
			wait := incrementalWait(1*time.Second, 2*time.Second)
			err = resource.Retry(10*time.Minute, func() *resource.RetryError {
				response, err := client.RpcPost("Rds", "2014-08-15", action, nil, request, false)
				if err != nil {
					if NeedRetry(err) {
						wait()
						return resource.RetryableError(err)
					}
					return resource.NonRetryableError(err)
				}
				addDebug(action, response, request)
				return nil
			})
			if err != nil {
				return WrapErrorf(err, DefaultErrorMsg, d.Id(), action, AlibabaCloudSdkGoERROR)
			}
		}

		if len(added) > 0 {
			action := "TagResources"
			request := map[string]interface{}{
				"RegionId":     s.client.RegionId,
				"ResourceType": "INSTANCE",
				"ResourceId.1": d.Id(),
			}
			count := 1
			for key, value := range added {
				request[fmt.Sprintf("Tag.%d.Key", count)] = key
				request[fmt.Sprintf("Tag.%d.Value", count)] = value
				count++
			}

			wait := incrementalWait(1*time.Second, 2*time.Second)
			err = resource.Retry(10*time.Minute, func() *resource.RetryError {
				response, err := client.RpcPost("Rds", "2014-08-15", action, nil, request, false)
				if err != nil {
					if NeedRetry(err) {
						wait()
						return resource.RetryableError(err)
					}
					return resource.NonRetryableError(err)
				}
				addDebug(action, response, request)
				return nil
			})
			if err != nil {
				return WrapErrorf(err, DefaultErrorMsg, d.Id(), action, AlibabaCloudSdkGoERROR)
			}
		}
		if err := s.WaitForDBInstance(d.Id(), Running, DefaultLongTimeout); err != nil {
			return WrapError(err)
		}
		d.SetPartial("tags")
	}

	return nil
}

func (s *RdsService) describeTags(d *schema.ResourceData) (tags []Tag, err error) {
	action := "DescribeTags"
	request := map[string]interface{}{
		"DBInstanceId": d.Id(),
		"RegionId":     s.client.RegionId,
		"SourceIp":     s.client.SourceIp,
	}
	client := s.client
	var response map[string]interface{}
	wait := incrementalWait(3*time.Second, 3*time.Second)
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		response, err = client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
		if err != nil {
			if NeedRetry(err) {
				wait()
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		addDebug(action, response, request)
		return nil
	})
	if err != nil {
		return nil, WrapErrorf(err, DefaultErrorMsg, d.Id(), action, AlibabaCloudSdkGoERROR)
	}
	return s.respToTags(response["Items"].(map[string]interface{})["TagInfos"].([]interface{})), nil
}

func (s *RdsService) respToTags(tagSet []interface{}) (tags []Tag) {
	result := make([]Tag, 0, len(tagSet))
	for _, t := range tagSet {
		t := t.(map[string]interface{})
		tag := Tag{
			Key:   t["TagKey"].(string),
			Value: t["TagValue"].(string),
		}
		result = append(result, tag)
	}

	return result
}

func (s *RdsService) tagsToMap(tags []Tag) map[string]string {
	result := make(map[string]string)
	for _, t := range tags {
		if !s.ignoreTag(t) {
			result[t.Key] = t.Value
		}
	}

	return result
}

func (s *RdsService) ignoreTag(t Tag) bool {
	filter := []string{"^aliyun", "^acs:", "^http://", "^https://"}
	for _, v := range filter {
		log.Printf("[DEBUG] Matching prefix %v with %v\n", v, t.Key)
		ok, _ := regexp.MatchString(v, t.Key)
		if ok {
			log.Printf("[DEBUG] Found Alibaba Cloud specific t %s (val: %s), ignoring.\n", t.Key, t.Value)
			return true
		}
	}
	return false
}

func (s *RdsService) tagsToString(tags []Tag) string {
	v, _ := json.Marshal(s.tagsToMap(tags))

	return string(v)
}

func (s *RdsService) DescribeDBProxy(id string) (object map[string]interface{}, err error) {
	action := "DescribeDBProxy"
	request := map[string]interface{}{
		"RegionId":     s.client.RegionId,
		"DBInstanceId": id,
		"SourceIp":     s.client.SourceIp,
	}
	client := s.client
	response, err := client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
	if err != nil {
		if IsExpectedErrors(err, []string{"InvalidDBInstanceId.NotFound"}) {
			return nil, WrapErrorf(err, NotFoundMsg, AlibabaCloudSdkGoERROR)
		}
		return nil, WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
	}
	addDebug(action, response, request)
	object = make(map[string]interface{})
	v, err := jsonpath.Get("$.DBProxyConnectStringItems", response)
	if err != nil {
		return object, WrapErrorf(err, FailedGetAttributeMsg, id, "$.DBProxyConnectStringItems", response)
	}
	if dbProxyInstanceType, ok := response["DBProxyInstanceType"]; ok {
		object["DBProxyInstanceType"] = dbProxyInstanceType
	}
	object["DBProxyInstanceStatus"] = response["DBProxyInstanceStatus"]
	if dBProxyInstanceNum, ok := response["DBProxyInstanceNum"]; ok {
		object["DBProxyInstanceNum"] = dBProxyInstanceNum
	}
	if dBProxyPersistentConnectionStatus, ok := response["DBProxyPersistentConnectionStatus"]; ok {
		object["DBProxyPersistentConnectionStatus"] = dBProxyPersistentConnectionStatus
	}
	if dBProxyInstanceCurrentMinorVersion, ok := response["DBProxyInstanceCurrentMinorVersion"]; ok {
		object["DBProxyInstanceCurrentMinorVersion"] = dBProxyInstanceCurrentMinorVersion
	}
	if dBProxyInstanceLatestMinorVersion, ok := response["DBProxyInstanceLatestMinorVersion"]; ok {
		object["DBProxyInstanceLatestMinorVersion"] = dBProxyInstanceLatestMinorVersion
	}
	if dBProxyServiceStatus, ok := response["DBProxyServiceStatus"]; ok {
		object["DBProxyServiceStatus"] = dBProxyServiceStatus
	}
	if dBProxyServiceStatus, ok := response["DBProxyInstanceName"]; ok {
		object["DBProxyInstanceName"] = dBProxyServiceStatus
	}
	if dBProxyConnectStringItems, ok := v.(map[string]interface{})["DBProxyConnectStringItems"].([]interface{}); ok {
		var innerItem, outerItem map[string]interface{}
		for _, item := range dBProxyConnectStringItems {
			if itemMap, ok := item.(map[string]interface{}); ok {
				netTypeStr, okStr := itemMap["DBProxyConnectStringNetType"].(string)
				netTypeInt, okInt := itemMap["DBProxyConnectStringNetWorkType"].(float64)

				if (okStr && netTypeStr == "InnerString") || (okInt && int(netTypeInt) == 2) {
					innerItem = itemMap
				} else if (okStr && netTypeStr == "OuterString") || (okInt && int(netTypeInt) == 0) {
					outerItem = itemMap
				}
			}
		}

		if innerItem == nil {
			return nil, WrapErrorf(NotFoundErr("DBProxyConnectStringItems", id), NotFoundMsg, ProviderERROR)
		}

		object["DBProxyVpcId"] = innerItem["DBProxyVpcId"]
		object["DBProxyVswitchId"] = innerItem["DBProxyVswitchId"]

		if outerItem != nil {
			object["DBProxyConnectString"] = outerItem["DBProxyConnectString"]
			object["DBProxyConnectStringPort"] = outerItem["DBProxyConnectStringPort"]
		} else {
			log.Printf("[WARN] No OuterString item found for resource %s", id)
		}
	}

	return object, nil
}

func (s *RdsService) DescribeDBProxyEndpoint(id string, endpointName string) (object map[string]interface{}, err error) {
	action := "DescribeDBProxyEndpoint"
	request := map[string]interface{}{
		"RegionId":          s.client.RegionId,
		"DBInstanceId":      id,
		"DBProxyEndpointId": endpointName,
		"SourceIp":          s.client.SourceIp,
	}
	client := s.client
	response, err := client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
	if err != nil {
		if IsExpectedErrors(err, []string{"InvalidDBInstanceId.NotFound", "Endpoint.NotFound"}) {
			return nil, WrapErrorf(err, NotFoundMsg, AlibabaCloudSdkGoERROR)
		}
		return nil, WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
	}
	addDebug(action, response, request)
	return response, nil
}

func (s *RdsService) DescribeRdsProxyEndpoint(id string) (object map[string]interface{}, err error) {
	action := "DescribeDBProxyEndpoint"
	request := map[string]interface{}{
		"RegionId":     s.client.RegionId,
		"DBInstanceId": id,
		"SourceIp":     s.client.SourceIp,
	}
	client := s.client
	response, err := client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
	if err != nil {
		if IsExpectedErrors(err, []string{"InvalidDBInstanceId.NotFound", "Endpoint.NotFound"}) {
			return nil, WrapErrorf(err, NotFoundMsg, AlibabaCloudSdkGoERROR)
		}
		return nil, WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
	}
	addDebug(action, response, request)
	return response, nil
}

func (s *RdsService) DescribeRdsParameterGroup(id string) (object map[string]interface{}, err error) {
	var response map[string]interface{}
	client := s.client
	action := "DescribeParameterGroup"
	request := map[string]interface{}{
		"RegionId":         s.client.RegionId,
		"ParameterGroupId": id,
	}
	response, err = client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
	if err != nil {
		if IsExpectedErrors(err, []string{"ParamGroupsNotExistError"}) {
			err = WrapErrorf(NotFoundErr("RdsParameterGroup", id), NotFoundMsg, ProviderERROR)
			return object, err
		}
		err = WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
		return object, err
	}
	addDebug(action, response, request)
	v, err := jsonpath.Get("$.ParamGroup.ParameterGroup", response)
	if err != nil {
		return object, WrapErrorf(err, FailedGetAttributeMsg, id, "$.ParamGroup.ParameterGroup", response)
	}
	if len(v.([]interface{})) < 1 {
		return object, WrapErrorf(NotFoundErr("RDS", id), NotFoundWithResponse, response)
	} else {
		if v.([]interface{})[0].(map[string]interface{})["ParameterGroupId"].(string) != id {
			return object, WrapErrorf(NotFoundErr("RDS", id), NotFoundWithResponse, response)
		}
	}
	object = v.([]interface{})[0].(map[string]interface{})
	return object, nil
}

func (s *RdsService) DescribeRdsAccount(id string) (object map[string]interface{}, err error) {
	var response map[string]interface{}
	client := s.client
	action := "DescribeAccounts"
	parts, err := ParseResourceId(id, 2)
	if err != nil {
		err = WrapError(err)
		return
	}
	request := map[string]interface{}{
		"RegionId":     s.client.RegionId,
		"SourceIp":     s.client.SourceIp,
		"AccountName":  parts[1],
		"DBInstanceId": parts[0],
	}
	response, err = client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
	if err != nil {
		if IsExpectedErrors(err, []string{"InvalidDBInstanceId.NotFound"}) {
			err = WrapErrorf(NotFoundErr("RdsAccount", id), NotFoundMsg, ProviderERROR)
			return object, err
		}
		err = WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
		return object, err
	}
	addDebug(action, response, request)
	v, err := jsonpath.Get("$.Accounts.DBInstanceAccount", response)
	if err != nil {
		return object, WrapErrorf(err, FailedGetAttributeMsg, id, "$.Accounts.DBInstanceAccount", response)
	}
	if len(v.([]interface{})) < 1 {
		return object, WrapErrorf(NotFoundErr("RDS", id), NotFoundWithResponse, response)
	}
	object = v.([]interface{})[0].(map[string]interface{})
	return object, nil
}

func (s *RdsService) RdsAccountStateRefreshFunc(id string, failStates []string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		object, err := s.DescribeRdsAccount(id)
		if err != nil {
			if NotFoundError(err) {
				// Set this to nil as if we didn't find anything.
				return nil, "", nil
			}
			return nil, "", WrapError(err)
		}
		for _, failState := range failStates {
			if object["AccountStatus"].(string) == failState {
				return object, object["AccountStatus"].(string), WrapError(Error(FailedToReachTargetStatus, object["AccountStatus"].(string)))
			}
		}
		return object, object["AccountStatus"].(string), nil
	}
}

func (s *RdsService) DescribeRdsBackup(id string) (object map[string]interface{}, err error) {
	var response map[string]interface{}
	client := s.client
	action := "DescribeBackups"
	parts, err := ParseResourceId(id, 2)
	if err != nil {
		err = WrapError(err)
		return
	}
	request := map[string]interface{}{
		"SourceIp":     s.client.SourceIp,
		"BackupId":     parts[1],
		"DBInstanceId": parts[0],
	}
	wait := incrementalWait(3*time.Second, 3*time.Second)
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		response, err = client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
		if err != nil {
			if NeedRetry(err) {
				wait()
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		return nil
	})
	addDebug(action, response, request)
	if err != nil {
		return object, WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
	}
	v, err := jsonpath.Get("$.Items.Backup", response)
	if err != nil {
		return object, WrapErrorf(err, FailedGetAttributeMsg, id, "$.Items.Backup", response)
	}
	if len(v.([]interface{})) < 1 {
		return object, WrapErrorf(NotFoundErr("RDS", id), NotFoundWithResponse, response)
	}
	object = v.([]interface{})[0].(map[string]interface{})
	return object, nil
}

func (s *RdsService) DescribeBackupTasks(id string, backupJobId string) (object map[string]interface{}, err error) {
	var response map[string]interface{}
	client := s.client
	action := "DescribeBackupTasks"
	request := map[string]interface{}{
		"SourceIp":     s.client.SourceIp,
		"DBInstanceId": id,
		"BackupJobId":  backupJobId,
	}
	wait := incrementalWait(3*time.Second, 3*time.Second)
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		response, err = client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
		if err != nil {
			if NeedRetry(err) {
				wait()
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		return nil
	})
	addDebug(action, response, request)
	if err != nil {
		return object, WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
	}
	v, err := jsonpath.Get("$.Items.BackupJob", response)
	if err != nil {
		return object, WrapErrorf(err, FailedGetAttributeMsg, id, "$.Items.BackupJob", response)
	}
	if len(v.([]interface{})) < 1 {
		return object, WrapErrorf(NotFoundErr("RDS", id), NotFoundWithResponse, response)
	} else {
		if fmt.Sprint(v.([]interface{})[0].(map[string]interface{})["BackupJobId"]) != backupJobId {
			return object, WrapErrorf(NotFoundErr("RDS", id), NotFoundWithResponse, response)
		}
	}
	object = v.([]interface{})[0].(map[string]interface{})
	return object, nil
}

func (s *RdsService) RdsBackupStateRefreshFunc(id string, backupJobId string, failStates []string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		object, err := s.DescribeBackupTasks(id, backupJobId)
		if err != nil {
			if NotFoundError(err) {
				// Set this to nil as if we didn't find anything.
				return nil, "", nil
			}
			return nil, "", WrapError(err)
		}

		for _, failState := range failStates {
			if object["BackupStatus"] == failState {
				return object, object["BackupStatus"].(string), WrapError(Error(FailedToReachTargetStatus, object["BackupStatus"]))
			}
		}
		return object, object["BackupStatus"].(string), nil
	}
}

func (s *RdsService) DescribeDBInstanceHAConfig(id string) (object map[string]interface{}, err error) {
	var response map[string]interface{}
	client := s.client
	action := "DescribeDBInstanceHAConfig"
	request := map[string]interface{}{
		"SourceIp":     s.client.SourceIp,
		"DBInstanceId": id,
	}
	wait := incrementalWait(3*time.Second, 3*time.Second)
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		response, err = client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
		if err != nil {
			if NeedRetry(err) {
				wait()
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		return nil
	})
	addDebug(action, response, request)
	if err != nil {
		return object, WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
	}
	v, err := jsonpath.Get("$", response)
	if err != nil {
		return object, WrapErrorf(err, FailedGetAttributeMsg, id, "$", response)
	}
	object = v.(map[string]interface{})
	return object, nil
}

func (s *RdsService) DescribeRdsCloneDbInstance(id string) (object map[string]interface{}, err error) {
	var response map[string]interface{}
	client := s.client
	action := "DescribeDBInstanceAttribute"
	request := map[string]interface{}{
		"SourceIp":     s.client.SourceIp,
		"DBInstanceId": id,
	}
	wait := incrementalWait(3*time.Second, 3*time.Second)
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		response, err = client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
		if err != nil {
			if NeedRetry(err) {
				wait()
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		return nil
	})
	addDebug(action, response, request)
	if err != nil {
		return object, WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
	}
	v, err := jsonpath.Get("$.Items.DBInstanceAttribute", response)
	if err != nil {
		return object, WrapErrorf(err, FailedGetAttributeMsg, id, "$.Items.DBInstanceAttribute", response)
	}
	if len(v.([]interface{})) < 1 {
		return object, WrapErrorf(NotFoundErr("RDS", id), NotFoundWithResponse, response)
	} else {
		if fmt.Sprint(v.([]interface{})[0].(map[string]interface{})["DBInstanceId"]) != id {
			return object, WrapErrorf(NotFoundErr("RDS", id), NotFoundWithResponse, response)
		}
	}
	object = v.([]interface{})[0].(map[string]interface{})
	return object, nil
}
func (s *RdsService) DescribePGHbaConfig(id string) (object map[string]interface{}, err error) {
	var response map[string]interface{}
	client := s.client
	action := "DescribePGHbaConfig"
	request := map[string]interface{}{
		"SourceIp":     s.client.SourceIp,
		"DBInstanceId": id,
	}
	wait := incrementalWait(3*time.Second, 3*time.Second)
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		response, err = client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
		if err != nil {
			if NeedRetry(err) {
				wait()
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		return nil
	})
	addDebug(action, response, request)
	if err != nil {
		return object, WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
	}
	v, err := jsonpath.Get("$", response)
	if err != nil {
		return object, WrapErrorf(err, FailedGetAttributeMsg, id, "$", response)
	}
	object = v.(map[string]interface{})
	return object, nil
}

func (s *RdsService) DescribeHADiagnoseConfig(id string) (object map[string]interface{}, err error) {
	var response map[string]interface{}
	client := s.client
	action := "DescribeHADiagnoseConfig"
	request := map[string]interface{}{
		"RegionId":     s.client.RegionId,
		"SourceIp":     s.client.SourceIp,
		"DBInstanceId": id,
	}
	wait := incrementalWait(3*time.Second, 3*time.Second)
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		response, err = client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
		if err != nil {
			if NeedRetry(err) {
				wait()
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		return nil
	})
	addDebug(action, response, request)
	if err != nil {
		return object, WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
	}
	v, err := jsonpath.Get("$", response)
	if err != nil {
		return object, WrapErrorf(err, FailedGetAttributeMsg, id, "$", response)
	}
	object = v.(map[string]interface{})
	return object, nil
}

func (s *RdsService) DescribeUpgradeMajorVersionPrecheckTask(id string, taskId int) (object map[string]interface{}, err error) {
	var response map[string]interface{}
	client := s.client
	action := "DescribeUpgradeMajorVersionPrecheckTask"
	request := map[string]interface{}{
		"SourceIp":     s.client.SourceIp,
		"DBInstanceId": id,
		"TaskId":       taskId,
	}
	wait := incrementalWait(3*time.Second, 3*time.Second)
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		response, err = client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
		if err != nil {
			if NeedRetry(err) {
				wait()
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		return nil
	})
	addDebug(action, response, request)
	if err != nil {
		return object, WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
	}
	v, err := jsonpath.Get("$.Items", response)
	if err != nil {
		return object, WrapErrorf(err, FailedGetAttributeMsg, id, "$.Items", response)
	}
	if len(v.([]interface{})) < 1 {
		return object, WrapErrorf(NotFoundErr("RDS", id), NotFoundWithResponse, response)
	} else {
		if formatInt(v.([]interface{})[0].(map[string]interface{})["TaskId"]) != taskId {
			return object, WrapErrorf(NotFoundErr("RDS", id), NotFoundWithResponse, response)
		}
	}
	object = v.([]interface{})[0].(map[string]interface{})
	return object, nil
}

func (s *RdsService) RdsUpgradeMajorVersionRefreshFunc(id string, taskId int, failStates []string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		object, err := s.DescribeUpgradeMajorVersionPrecheckTask(id, taskId)
		if err != nil {
			if NotFoundError(err) {
				// Set this to nil as if we didn't find anything.
				return nil, "", nil
			}
			return nil, "", WrapError(err)
		}

		for _, failState := range failStates {
			if object["Result"] == failState {
				return object, object["Result"].(string), WrapError(Error(FailedToReachTargetStatus, object["Result"]))
			}
		}
		return object, object["Result"].(string), nil
	}
}

func (s *RdsService) DescribeRdsServiceLinkedRole(id string) (*ram.GetRoleResponse, error) {
	response := &ram.GetRoleResponse{}
	request := ram.CreateGetRoleRequest()
	request.RegionId = s.client.RegionId
	request.RoleName = id
	err := resource.Retry(5*time.Minute, func() *resource.RetryError {
		raw, err := s.client.WithRamClient(func(ramClient *ram.Client) (interface{}, error) {
			return ramClient.GetRole(request)
		})
		if err != nil {
			if IsExpectedErrors(err, []string{ThrottlingUser}) {
				time.Sleep(2 * time.Second)
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		addDebug(request.GetActionName(), raw, request.RpcRequest, request)
		response, _ = raw.(*ram.GetRoleResponse)
		return nil
	})
	if err != nil {
		if IsExpectedErrors(err, []string{"EntityNotExist.Role"}) {
			return response, WrapErrorf(err, NotFoundMsg, AlibabaCloudSdkGoERROR)
		}
		return response, WrapErrorf(err, DefaultErrorMsg, id, request.GetActionName(), AlibabaCloudSdkGoERROR)
	}
	return response, nil
}

func (s *RdsService) RdsDBProxyStateRefreshFunc(id string, failStates []string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		object, err := s.DescribeDBProxy(id)
		if err != nil {
			if NotFoundError(err) {
				// Set this to nil as if we didn't find anything.
				return nil, "", nil
			}
			return nil, "", WrapError(err)
		}
		for _, failState := range failStates {
			if object["DBProxyInstanceStatus"] == failState {
				return object, fmt.Sprint(object["DBProxyInstanceStatus"]), WrapError(Error(FailedToReachTargetStatus, object["DBProxyInstanceStatus"]))
			}
		}
		return object, fmt.Sprint(object["DBProxyInstanceStatus"]), nil
	}
}

func (s *RdsService) GetDbProxyInstanceSsl(id string) (object map[string]interface{}, err error) {
	action := "GetDbProxyInstanceSsl"
	request := map[string]interface{}{
		"RegionId":     s.client.RegionId,
		"DBInstanceId": id,
		"SourceIp":     s.client.SourceIp,
	}
	client := s.client
	response, err := client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
	if err != nil {
		if IsExpectedErrors(err, []string{"InvalidDBInstanceId.NotFound"}) {
			return object, WrapErrorf(NotFoundErr("RDS", id), NotFoundWithResponse, response)
		}
		return object, WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
	}
	addDebug(action, response, request)
	v, err := jsonpath.Get("$.DbProxyCertListItems.DbProxyCertListItems", response)
	if err != nil {
		return object, WrapErrorf(NotFoundErr("RDS", id), NotFoundWithResponse, response)
	}
	if len(v.([]interface{})) < 1 {
		return object, nil
	}
	return v.([]interface{})[0].(map[string]interface{}), nil
}

func (s *RdsService) DescribeInstanceCrossBackupPolicy(id string) (object map[string]interface{}, err error) {
	action := "DescribeInstanceCrossBackupPolicy"
	request := map[string]interface{}{
		"RegionId":     s.client.RegionId,
		"DBInstanceId": id,
		"SourceIp":     s.client.SourceIp,
	}
	client := s.client
	response, err := client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
	if err != nil {
		if IsExpectedErrors(err, []string{"InvalidDBInstanceId.NotFound"}) {
			return nil, WrapErrorf(err, NotFoundMsg, AlibabaCloudSdkGoERROR)
		}
		return nil, WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
	}
	if v, ok := response["BackupEnabled"]; ok && v.(string) == "Disabled" {
		return nil, WrapErrorf(err, NotFoundMsg, AlibabaCloudSdkGoERROR)
	}
	addDebug(action, response, request)
	return response, nil
}

func (s *RdsService) FindKmsRoleArnDdr(k string) (string, error) {
	action := "DescribeKey"
	var response map[string]interface{}
	var err error
	client := s.client
	request := make(map[string]interface{})
	request["KeyId"] = k
	wait := incrementalWait(3*time.Second, 3*time.Second)
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		response, err = client.RpcPost("Kms", "2016-01-20", action, nil, request, true)
		if err != nil {
			if NeedRetry(err) {
				wait()
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		return nil
	})
	addDebug(action, response, request)
	if err != nil {
		return "", WrapErrorf(err, DataDefaultErrorMsg, k, action, AlibabaCloudSdkGoERROR)
	}
	resp, err := jsonpath.Get("$.KeyMetadata.Creator", response)
	if err != nil {
		return "", WrapErrorf(err, FailedGetAttributeMsg, action, "$.VersionIds.VersionId", response)
	}
	return strings.Join([]string{"acs:ram::", fmt.Sprint(resp), ":role/aliyunrdsinstanceencryptiondefaultrole"}, ""), nil
}

func (s *RdsService) DescribeRdsNode(id string) (object map[string]interface{}, err error) {
	client := s.client
	parts, err := ParseResourceId(id, 2)
	if err != nil {
		return nil, WrapError(err)
	}
	action := "DescribeDBInstanceAttribute"
	request := map[string]interface{}{
		"RegionId":     s.client.RegionId,
		"DBInstanceId": parts[0],
		"SourceIp":     s.client.SourceIp,
	}
	var response map[string]interface{}
	wait := incrementalWait(3*time.Second, 3*time.Second)
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		response, err = client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
		if err != nil {
			if NeedRetry(err) {
				wait()
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		addDebug(action, response, request)
		return nil
	})
	if err != nil {
		if IsExpectedErrors(err, []string{"InvalidDBInstanceId.NotFound"}) {
			return nil, WrapErrorf(err, NotFoundMsg, AlibabaCloudSdkGoERROR)
		}
		return nil, WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
	}
	v, err := jsonpath.Get("$.Items.DBInstanceAttribute", response)
	if err != nil {
		return nil, WrapErrorf(err, FailedGetAttributeMsg, id, "$.Items.DBInstanceAttribute", response)
	}
	if len(v.([]interface{})) < 1 {
		return nil, WrapErrorf(NotFoundErr("DBAccount", id), NotFoundMsg, ProviderERROR)
	}

	dbNodeList := v.([]interface{})[0].(map[string]interface{})
	if dbNodesList, ok := dbNodeList["DBClusterNodes"]; ok && dbNodesList != nil {
		if nodeList, ok := dbNodesList.(map[string]interface{})["DBClusterNode"].([]interface{}); ok {
			if len(nodeList) < 3 {
				return nil, WrapErrorf(NotFoundErr("DBAccount", id), NotFoundMsg, ProviderERROR)
			}
			for _, node := range nodeList {
				nodeId := node.(map[string]interface{})["NodeId"]
				if nodeId.(string) == parts[1] {
					object = node.(map[string]interface{})
					object["DBInstanceId"] = dbNodeList["DBInstanceId"]
					break
				}
			}
		}
	}
	return object, nil
}

func (s *RdsService) DescribeDBInstanceEndpoints(id string) (object map[string]interface{}, err error) {
	var response map[string]interface{}
	client := s.client
	parts, err := ParseResourceId(id, 2)
	if err != nil {
		return nil, WrapError(err)
	}
	_, err = s.DescribeDBInstance(parts[0])
	if err != nil {
		return nil, WrapError(err)
	}
	action := "DescribeDBInstanceEndpoints"
	request := map[string]interface{}{
		"SourceIp":             s.client.SourceIp,
		"DBInstanceId":         parts[0],
		"DBInstanceEndpointId": parts[1],
		"RegionId":             s.client.RegionId,
	}
	wait := incrementalWait(3*time.Second, 3*time.Second)
	err = resource.Retry(3*time.Minute, func() *resource.RetryError {
		response, err = client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
		if err != nil {
			if NeedRetry(err) {
				wait()
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		return nil
	})
	addDebug(action, response, request)
	if err != nil {
		if IsExpectedErrors(err, []string{"InvalidDBInstance.NotFound"}) {
			return nil, WrapErrorf(err, NotFoundMsg, AlibabaCloudSdkGoERROR)
		}
		return object, WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
	}
	v, err := jsonpath.Get("$.Data.DBInstanceEndpoints", response)
	if err != nil {
		return object, WrapErrorf(err, FailedGetAttributeMsg, id, "$.Data.DBInstanceEndpoints", response)
	}
	if endpoints, ok := v.(map[string]interface{})["DBInstanceEndpoint"].([]interface{}); ok {
		if len(endpoints) < 1 {
			return nil, WrapErrorf(NotFoundErr("DBInstanceEndpoint", id), NotFoundMsg, ProviderERROR)
		}
		endpoint := endpoints[0].(map[string]interface{})
		object = make(map[string]interface{})
		object["EndpointDescription"] = endpoint["EndpointDescription"]
		object["EndpointType"] = endpoint["EndpointType"]
		object["DBInstanceId"] = parts[0]
		object["DBInstanceEndpointId"] = parts[1]
		nodeList := endpoint["NodeItems"]
		nodeItems := nodeList.(map[string]interface{})["NodeItem"].([]interface{})
		dbNodesMaps := make([]map[string]interface{}, 0)
		if nodeItems != nil && len(nodeItems) > 0 {
			for _, nodeItem := range nodeItems {
				dbNodesListItemMap := map[string]interface{}{}
				dbNodesListItemMap["node_id"] = nodeItem.(map[string]interface{})["NodeId"]
				dbNodesListItemMap["weight"] = nodeItem.(map[string]interface{})["Weight"]
				dbNodesMaps = append(dbNodesMaps, dbNodesListItemMap)
			}
			object["NodeItems"] = dbNodesMaps
		}
		addressList := endpoint["AddressItems"]
		addressItems := addressList.(map[string]interface{})["AddressItem"].([]interface{})
		if addressItems != nil && len(addressItems) > 0 {
			for _, addressItem := range addressItems {
				ipType := addressItem.(map[string]interface{})["IpType"]
				if ipType.(string) == "Private" {
					object["VpcId"] = addressItem.(map[string]interface{})["VpcId"]
					object["IpType"] = addressItem.(map[string]interface{})["IpType"]
					object["IpAddress"] = addressItem.(map[string]interface{})["IpAddress"]
					object["VSwitchId"] = addressItem.(map[string]interface{})["VSwitchId"]
					object["Port"] = addressItem.(map[string]interface{})["Port"]
					object["ConnectionString"] = addressItem.(map[string]interface{})["ConnectionString"]
					object["ConnectionStringPrefix"] = strings.Split(fmt.Sprint(object["ConnectionString"]), ".")[0]
					break
				}
			}
		}
	}
	return object, nil
}

func (s *RdsService) DescribeDBInstanceEndpointPublicAddress(id string) (object map[string]interface{}, err error) {
	var response map[string]interface{}
	client := s.client
	parts, err := ParseResourceId(id, 2)
	if err != nil {
		return nil, WrapError(err)
	}
	action := "DescribeDBInstanceEndpoints"
	request := map[string]interface{}{
		"SourceIp":             s.client.SourceIp,
		"DBInstanceId":         parts[0],
		"DBInstanceEndpointId": parts[1],
		"RegionId":             s.client.RegionId,
	}
	wait := incrementalWait(3*time.Second, 3*time.Second)
	err = resource.Retry(3*time.Minute, func() *resource.RetryError {
		response, err = client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
		if err != nil {
			if NeedRetry(err) {
				wait()
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		return nil
	})
	addDebug(action, response, request)
	if err != nil {
		if IsExpectedErrors(err, []string{"InvalidDBInstance.NotFound"}) {
			return nil, WrapErrorf(err, NotFoundMsg, AlibabaCloudSdkGoERROR)
		}
		return nil, WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
	}
	v, err := jsonpath.Get("$.Data.DBInstanceEndpoints", response)
	if err != nil {
		return object, WrapErrorf(err, FailedGetAttributeMsg, id, "$.Data.DBInstanceEndpoints", response)
	}
	if endpoints, ok := v.(map[string]interface{})["DBInstanceEndpoint"].([]interface{}); ok {
		if len(endpoints) < 1 {
			return nil, WrapErrorf(NotFoundErr("DBInstanceEndpoint", id), NotFoundMsg, ProviderERROR)
		}
		endpoint := endpoints[0].(map[string]interface{})
		object = make(map[string]interface{})
		object["DBInstanceId"] = parts[0]
		object["DBInstanceEndpointId"] = parts[1]
		addressList := endpoint["AddressItems"]
		addressItems := addressList.(map[string]interface{})["AddressItem"].([]interface{})
		if addressItems != nil && len(addressItems) > 0 {
			for _, addressItem := range addressItems {
				ipType := addressItem.(map[string]interface{})["IpType"]
				if ipType.(string) == "Public" {
					object["IpType"] = addressItem.(map[string]interface{})["IpType"]
					object["IpAddress"] = addressItem.(map[string]interface{})["IpAddress"]
					object["Port"] = addressItem.(map[string]interface{})["Port"]
					object["ConnectionString"] = addressItem.(map[string]interface{})["ConnectionString"]
					object["ConnectionStringPrefix"] = strings.Split(fmt.Sprint(object["ConnectionString"]), ".")[0]
					break
				}
			}
		}
		if _, ok := object["IpType"]; !ok {
			return nil, WrapErrorf(NotFoundErr("DBInstanceEndpointPublicAddress", id), NotFoundMsg, ProviderERROR)
		}
	}
	return object, nil
}

func (s *RdsService) DescribeAllWhitelistTemplate(name string) (object map[string]interface{}, err error) {
	action := "DescribeAllWhitelistTemplate"
	request := map[string]interface{}{
		"RegionId":          s.client.RegionId,
		"MaxRecordsPerPage": 1000,
		"PageNumbers":       1,
	}
	client := s.client
	response, err := client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
	if err != nil {
		if IsExpectedErrors(err, []string{}) {
			return nil, WrapErrorf(err, NotFoundMsg, AlibabaCloudSdkGoERROR)
		}
		return nil, WrapErrorf(err, DefaultErrorMsg, action, AlibabaCloudSdkGoERROR)
	}
	addDebug(action, response, request)
	object = make(map[string]interface{})
	v, err := jsonpath.Get("$.Data.Templates", response)
	if err != nil {
		return object, WrapErrorf(err, FailedGetAttributeMsg, action, "$.Data.Templates", response)
	}
	if templatesList, ok := v.([]interface{}); ok {
		for _, item := range templatesList {
			if itemMap, ok := item.(map[string]interface{}); ok {
				templateName, ok := itemMap["TemplateName"].(string)
				if !ok {
					continue
				}

				if templateName == name {
					object["UserId"] = itemMap["UserId"]
					object["TemplateName"] = itemMap["TemplateName"]
					object["Id"] = itemMap["Id"]
					object["Ips"] = itemMap["Ips"]
					object["TemplateId"] = itemMap["TemplateId"]
					return object, nil
				}
			} else {
				log.Printf("[WARN] Unexpected template item type: %T", item)
			}
		}
	} else {
		log.Printf("[WARN] Templates is not an array: %T", v)
	}
	return nil, WrapErrorf(NotFoundErr("TemplateName", "test02"), NotFoundMsg, ProviderERROR)
}

func (s *RdsService) DescribeWhitelistTemplate(name string) (object map[string]interface{}, err error) {
	action := "DescribeWhitelistTemplate"
	request := map[string]interface{}{
		"RegionId":   s.client.RegionId,
		"TemplateId": name,
	}
	client := s.client
	response, err := client.RpcPost("Rds", "2014-08-15", action, nil, request, true)
	if err != nil {
		if IsExpectedErrors(err, []string{}) {
			return nil, WrapErrorf(err, NotFoundMsg, AlibabaCloudSdkGoERROR)
		}
		return nil, WrapErrorf(err, DefaultErrorMsg, action, AlibabaCloudSdkGoERROR)
	}
	addDebug(action, response, request)
	object = make(map[string]interface{})
	v, err := jsonpath.Get("$.Data.Template", response)
	if err != nil {
		return object, WrapErrorf(err, FailedGetAttributeMsg, action, "$.Data.Template", response)
	}
	templateMap, ok := v.(map[string]interface{})
	if !ok {
		return nil, WrapErrorf(Error("Failed to parse Template as map"), FailedGetAttributeMsg, action, "$.Data.Template", response)
	}

	object["UserId"] = templateMap["UserId"]
	object["TemplateName"] = templateMap["TemplateName"]
	object["Id"] = templateMap["Id"]
	object["Ips"] = templateMap["Ips"]
	object["TemplateId"] = templateMap["TemplateId"]

	return object, nil
}
