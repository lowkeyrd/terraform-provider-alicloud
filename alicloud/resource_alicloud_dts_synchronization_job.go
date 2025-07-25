package alicloud

import (
	"fmt"
	"log"
	"time"

	"github.com/aliyun/terraform-provider-alicloud/alicloud/connectivity"
	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

func resourceAlicloudDtsSynchronizationJob() *schema.Resource {
	return &schema.Resource{
		Create: resourceAlicloudDtsSynchronizationJobCreate,
		Read:   resourceAlicloudDtsSynchronizationJobRead,
		Update: resourceAlicloudDtsSynchronizationJobUpdate,
		Delete: resourceAlicloudDtsSynchronizationJobDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		Timeouts: &schema.ResourceTimeout{
			Update: schema.DefaultTimeout(10 * time.Minute),
		},
		Schema: map[string]*schema.Schema{
			"dts_instance_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"dts_job_name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"dedicated_cluster_id": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"data_check_configure": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"checkpoint": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},
			"instance_class": {
				Type:         schema.TypeString,
				Optional:     true,
				Computed:     true,
				ValidateFunc: StringInSlice([]string{"4xlarge", "2xlarge", "xlarge", "large", "medium", "small"}, false),
			},
			"data_initialization": {
				Type:     schema.TypeBool,
				Required: true,
				ForceNew: true,
			},
			"data_synchronization": {
				Type:     schema.TypeBool,
				Required: true,
				ForceNew: true,
			},
			"structure_initialization": {
				Type:     schema.TypeBool,
				Required: true,
				ForceNew: true,
			},
			"synchronization_direction": {
				Type:         schema.TypeString,
				Optional:     true,
				Computed:     true,
				ForceNew:     true,
				ValidateFunc: StringInSlice([]string{"Forward", "Reverse"}, false),
			},
			"db_list": {
				Type:     schema.TypeString,
				Required: true,
			},
			"reserve": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"source_endpoint_instance_type": {
				Type:             schema.TypeString,
				Required:         true,
				ForceNew:         true,
				ValidateFunc:     StringInSlice([]string{"CEN", "DG", "DISTRIBUTED_DMSLOGICDB", "ECS", "EXPRESS", "MONGODB", "OTHER", "PolarDB", "POLARDBX20", "RDS", "REDIS", "DISTRIBUTED_POLARDBX10"}, true),
				DiffSuppressFunc: UpperLowerCaseDiffSuppressFunc,
			},
			"source_endpoint_engine_name": {
				Type:             schema.TypeString,
				Required:         true,
				ForceNew:         true,
				ValidateFunc:     StringInSlice([]string{"AS400", "DB2", "DMSPOLARDB", "HBASE", "MONGODB", "MSSQL", "MySQL", "ORACLE", "PolarDB", "POLARDBX20", "POLARDB_O", "POSTGRESQL", "TERADATA", "POLARDB_PG", "MARIADB", "POLARDBX10", "TiDB", "REDIS"}, true),
				DiffSuppressFunc: UpperLowerCaseDiffSuppressFunc,
			},
			"source_endpoint_instance_id": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"source_endpoint_region": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"source_endpoint_ip": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"source_endpoint_port": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"source_endpoint_oracle_sid": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"source_endpoint_database_name": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"source_endpoint_user_name": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"source_endpoint_password": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"source_endpoint_owner_id": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"source_endpoint_role": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"source_endpoint_vswitch_id": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"destination_endpoint_instance_type": {
				Type:             schema.TypeString,
				Required:         true,
				ForceNew:         true,
				ValidateFunc:     StringInSlice([]string{"ads", "ADS", "CEN", "DATAHUB", "DG", "ECS", "EXPRESS", "GREENPLUM", "MONGODB", "OTHER", "PolarDB", "POLARDBX20", "RDS", "REDIS", "ELK", "Tablestore", "ODPS"}, true),
				DiffSuppressFunc: UpperLowerCaseDiffSuppressFunc,
			},
			"destination_endpoint_engine_name": {
				Type:             schema.TypeString,
				Required:         true,
				ForceNew:         true,
				ValidateFunc:     StringInSlice([]string{"ADB20", "ADS", "ADB30", "AS400", "DATAHUB", "DB2", "GREENPLUM", "KAFKA", "MONGODB", "MSSQL", "MySQL", "ORACLE", "PolarDB", "POLARDBX20", "POLARDB_O", "PostgreSQL", "POLARDB_PG", "MARIADB", "POLARDBX10", "ODPS", "Tablestore", "ELK", "REDIS"}, true),
				DiffSuppressFunc: UpperLowerCaseDiffSuppressFunc,
			},
			"destination_endpoint_instance_id": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"destination_endpoint_region": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"destination_endpoint_ip": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"destination_endpoint_port": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"destination_endpoint_database_name": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"destination_endpoint_user_name": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"destination_endpoint_password": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"destination_endpoint_oracle_sid": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"destination_endpoint_owner_id": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"destination_endpoint_role": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"dts_bis_label": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"delay_notice": {
				Type:     schema.TypeBool,
				Optional: true,
				ForceNew: true,
			},
			"delay_phone": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"delay_rule_time": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"error_notice": {
				Type:     schema.TypeBool,
				Optional: true,
				ForceNew: true,
			},
			"error_phone": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"status": {
				Type:         schema.TypeString,
				Optional:     true,
				Computed:     true,
				ValidateFunc: StringInSlice([]string{"Synchronizing", "Suspending"}, false),
			},
		},
	}
}

func resourceAlicloudDtsSynchronizationJobCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*connectivity.AliyunClient)
	var response map[string]interface{}
	action := "ConfigureDtsJob"
	request := make(map[string]interface{})
	var err error
	if v, ok := d.GetOk("dts_instance_id"); ok {
		request["DtsInstanceId"] = v
	}
	request["DtsJobName"] = d.Get("dts_job_name")
	if v, ok := d.GetOk("checkpoint"); ok {
		request["Checkpoint"] = v
	}
	request["DataInitialization"] = d.Get("data_initialization")
	request["DataSynchronization"] = d.Get("data_synchronization")
	request["StructureInitialization"] = d.Get("structure_initialization")
	request["SynchronizationDirection"] = d.Get("synchronization_direction")
	request["DbList"] = d.Get("db_list")
	if v, ok := d.GetOkExists("delay_notice"); ok {
		request["DelayNotice"] = v
	}
	if v, ok := d.GetOk("delay_phone"); ok {
		request["DelayPhone"] = v
	}
	if v, ok := d.GetOk("delay_rule_time"); ok {
		request["DelayRuleTime"] = v
	}
	if v, ok := d.GetOk("destination_endpoint_database_name"); ok {
		request["DestinationEndpointDataBaseName"] = v
	}
	if v, ok := d.GetOk("destination_endpoint_engine_name"); ok {
		request["DestinationEndpointEngineName"] = v
	}
	if v, ok := d.GetOk("destination_endpoint_ip"); ok {
		request["DestinationEndpointIP"] = v
	}
	if v, ok := d.GetOk("destination_endpoint_instance_id"); ok {
		request["DestinationEndpointInstanceID"] = v
	}
	request["DestinationEndpointInstanceType"] = d.Get("destination_endpoint_instance_type")
	if v, ok := d.GetOk("destination_endpoint_oracle_sid"); ok {
		request["DestinationEndpointOracleSID"] = v
	}
	if v, ok := d.GetOk("destination_endpoint_password"); ok {
		request["DestinationEndpointPassword"] = v
	}
	if v, ok := d.GetOk("destination_endpoint_port"); ok {
		request["DestinationEndpointPort"] = v
	}
	if v, ok := d.GetOk("destination_endpoint_region"); ok {
		request["DestinationEndpointRegion"] = v
	}
	if v, ok := d.GetOk("destination_endpoint_user_name"); ok {
		request["DestinationEndpointUserName"] = v
	}
	if v, ok := d.GetOkExists("error_notice"); ok {
		request["ErrorNotice"] = v
	}
	if v, ok := d.GetOk("error_phone"); ok {
		request["ErrorPhone"] = v
	}
	request["JobType"] = "SYNC"
	request["RegionId"] = client.RegionId
	if v, ok := d.GetOk("reserve"); ok {
		request["Reserve"] = v
	}
	if v, ok := d.GetOk("source_endpoint_database_name"); ok {
		request["SourceEndpointDatabaseName"] = v
	}
	if v, ok := d.GetOk("source_endpoint_engine_name"); ok {
		request["SourceEndpointEngineName"] = v
	}
	if v, ok := d.GetOk("source_endpoint_ip"); ok {
		request["SourceEndpointIP"] = v
	}
	if v, ok := d.GetOk("source_endpoint_instance_id"); ok {
		request["SourceEndpointInstanceID"] = v
	}
	request["SourceEndpointInstanceType"] = d.Get("source_endpoint_instance_type")
	if v, ok := d.GetOk("source_endpoint_oracle_sid"); ok {
		request["SourceEndpointOracleSID"] = v
	}
	if v, ok := d.GetOk("source_endpoint_owner_id"); ok {
		request["SourceEndpointOwnerID"] = v
	}
	if v, ok := d.GetOk("source_endpoint_password"); ok {
		request["SourceEndpointPassword"] = v
	}
	if v, ok := d.GetOk("source_endpoint_port"); ok {
		request["SourceEndpointPort"] = v
	}
	if v, ok := d.GetOk("source_endpoint_region"); ok {
		request["SourceEndpointRegion"] = v
	}
	if v, ok := d.GetOk("source_endpoint_role"); ok {
		request["SourceEndpointRole"] = v
	}
	if v, ok := d.GetOk("source_endpoint_user_name"); ok {
		request["SourceEndpointUserName"] = v
	}
	if v, ok := d.GetOk("dedicated_cluster_id"); ok {
		request["DedicatedClusterId"] = v
	}
	if v, ok := d.GetOk("data_check_configure"); ok {
		request["DataCheckConfigure"] = v
	}
	if v, ok := d.GetOk("source_endpoint_vswitch_id"); ok {
		request["SourceEndpointVSwitchID"] = v
	}
	if v, ok := d.GetOk("destination_endpoint_owner_id"); ok {
		request["DestinationEndpointOwnerID"] = v
	}
	if v, ok := d.GetOk("destination_endpoint_role"); ok {
		request["DestinationEndpointRole"] = v
	}
	if v, ok := d.GetOk("dts_bis_label"); ok {
		request["DtsBisLabel"] = v
	}
	wait := incrementalWait(3*time.Second, 3*time.Second)
	err = resource.Retry(d.Timeout(schema.TimeoutCreate), func() *resource.RetryError {
		response, err = client.RpcPost("Dts", "2020-01-01", action, nil, request, false)
		if err != nil {
			if IsExpectedErrors(err, []string{"SQLExecuteError"}) || NeedRetry(err) {
				wait()
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		return nil
	})
	addDebug(action, response, request)
	if err != nil {
		return WrapErrorf(err, DefaultErrorMsg, "alicloud_dts_synchronization_job", action, AlibabaCloudSdkGoERROR)
	}

	d.SetId(fmt.Sprint(response["DtsJobId"]))
	d.Set("dts_instance_id", response["DtsInstanceId"])
	dtsService := DtsService{client}
	stateConf := BuildStateConf([]string{}, []string{"Synchronizing", "NotStarted"}, d.Timeout(schema.TimeoutCreate), 5*time.Second, dtsService.DtsSynchronizationJobStateRefreshFunc(d.Id(), []string{"PrecheckFailed", "InitializeFailed", "Failed"}))
	if _, err := stateConf.WaitForState(); err != nil {
		return WrapErrorf(err, IdMsg, d.Id())
	}

	return resourceAlicloudDtsSynchronizationJobUpdate(d, meta)
}
func resourceAlicloudDtsSynchronizationJobRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*connectivity.AliyunClient)
	dtsService := DtsService{client}
	object, err := dtsService.DescribeDtsSynchronizationJob(d.Id())
	if err != nil {
		if NotFoundError(err) {
			log.Printf("[DEBUG] Resource alicloud_dts_synchronization_job dtsService.DescribeDtsSynchronizationJob Failed!!! %s", err)
			d.SetId("")
			return nil
		}
		return WrapError(err)
	}
	migrationModeObj := object["MigrationMode"].(map[string]interface{})
	destinationEndpointObj := object["DestinationEndpoint"].(map[string]interface{})
	sourceEndpointObj := object["SourceEndpoint"].(map[string]interface{})
	d.Set("checkpoint", fmt.Sprint(formatInt(object["Checkpoint"])))
	d.Set("data_initialization", migrationModeObj["DataInitialization"])
	d.Set("data_synchronization", migrationModeObj["DataSynchronization"])
	d.Set("db_list", object["DbObject"])
	d.Set("destination_endpoint_database_name", destinationEndpointObj["DatabaseName"])
	d.Set("destination_endpoint_engine_name", destinationEndpointObj["EngineName"])
	d.Set("destination_endpoint_ip", destinationEndpointObj["Ip"])
	d.Set("destination_endpoint_instance_id", destinationEndpointObj["InstanceID"])
	d.Set("destination_endpoint_instance_type", destinationEndpointObj["InstanceType"])
	d.Set("destination_endpoint_oracle_sid", destinationEndpointObj["OracleSID"])
	d.Set("destination_endpoint_port", destinationEndpointObj["Port"])
	d.Set("destination_endpoint_region", destinationEndpointObj["Region"])
	d.Set("destination_endpoint_user_name", destinationEndpointObj["UserName"])
	d.Set("dts_instance_id", object["DtsInstanceID"])
	d.Set("dts_job_name", object["DtsJobName"])
	d.Set("source_endpoint_database_name", sourceEndpointObj["DatabaseName"])
	d.Set("source_endpoint_engine_name", convertSourceEndpointEngineNameUppercaseResponse(sourceEndpointObj["EngineName"]))
	d.Set("source_endpoint_ip", sourceEndpointObj["Ip"])
	d.Set("source_endpoint_instance_id", sourceEndpointObj["InstanceID"])
	d.Set("source_endpoint_instance_type", sourceEndpointObj["InstanceType"])
	d.Set("source_endpoint_oracle_sid", sourceEndpointObj["OracleSID"])
	d.Set("source_endpoint_owner_id", sourceEndpointObj["AliyunUid"])
	d.Set("source_endpoint_port", sourceEndpointObj["Port"])
	d.Set("source_endpoint_region", sourceEndpointObj["Region"])
	d.Set("source_endpoint_role", sourceEndpointObj["RoleName"])
	d.Set("source_endpoint_user_name", sourceEndpointObj["UserName"])
	d.Set("status", object["Status"])
	d.Set("structure_initialization", migrationModeObj["StructureInitialization"])
	d.Set("synchronization_direction", object["SynchronizationDirection"])

	return nil
}
func resourceAlicloudDtsSynchronizationJobUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*connectivity.AliyunClient)
	var response map[string]interface{}
	var err error
	d.Partial(true)

	update := false
	request := map[string]interface{}{
		"DtsJobId": d.Id(),
	}
	if !d.IsNewResource() && d.HasChange("dts_job_name") {
		update = true
		request["DtsJobName"] = d.Get("dts_job_name")
	}
	request["RegionId"] = client.RegionId
	if update {
		action := "ModifyDtsJobName"
		wait := incrementalWait(3*time.Second, 3*time.Second)
		err = resource.Retry(d.Timeout(schema.TimeoutUpdate), func() *resource.RetryError {
			response, err = client.RpcPost("Dts", "2020-01-01", action, nil, request, false)
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
			return WrapErrorf(err, DefaultErrorMsg, d.Id(), action, AlibabaCloudSdkGoERROR)
		}
		if fmt.Sprint(response["Success"]) == "false" {
			return WrapError(fmt.Errorf("%s failed, response: %v", action, response))
		}
		d.SetPartial("dts_job_name")
	}

	modifyDtsJobPasswordReq := map[string]interface{}{
		"DtsJobId": d.Id(),
	}
	modifyDtsJobPasswordReq["RegionId"] = client.RegionId
	if !d.IsNewResource() && d.HasChange("source_endpoint_password") {

		modifyDtsJobPasswordReq["Endpoint"] = "src"
		if v, ok := d.GetOk("source_endpoint_password"); ok {
			modifyDtsJobPasswordReq["Password"] = v
		}
		if v, ok := d.GetOk("source_endpoint_user_name"); ok {
			modifyDtsJobPasswordReq["UserName"] = v
		}

		action := "ModifyDtsJobPassword"
		wait := incrementalWait(3*time.Second, 3*time.Second)
		err = resource.Retry(d.Timeout(schema.TimeoutUpdate), func() *resource.RetryError {
			response, err = client.RpcPost("Dts", "2020-01-01", action, nil, modifyDtsJobPasswordReq, false)
			if err != nil {
				if NeedRetry(err) {
					wait()
					return resource.RetryableError(err)
				}
				return resource.NonRetryableError(err)
			}
			return nil
		})
		addDebug(action, response, modifyDtsJobPasswordReq)
		if err != nil {
			return WrapErrorf(err, DefaultErrorMsg, d.Id(), action, AlibabaCloudSdkGoERROR)
		}
		if fmt.Sprint(response["Success"]) == "false" {
			return WrapError(fmt.Errorf("%s failed, response: %v", action, response))
		}
		d.SetPartial("source_endpoint_password")
		d.SetPartial("source_endpoint_user_name")

		target := d.Get("status").(string)
		err = resourceAlicloudDtsSynchronizationJobStatusFlow(d, meta, target)
		if err != nil {
			return WrapError(Error(FailedToReachTargetStatus, d.Get("status")))
		}
	}

	if !d.IsNewResource() && d.HasChange("destination_endpoint_password") {

		modifyDtsJobPasswordReq["Endpoint"] = "dest"
		if v, ok := d.GetOk("destination_endpoint_password"); ok {
			modifyDtsJobPasswordReq["Password"] = v
		}
		if v, ok := d.GetOk("destination_endpoint_user_name"); ok {
			modifyDtsJobPasswordReq["UserName"] = v
		}

		action := "ModifyDtsJobPassword"
		wait := incrementalWait(3*time.Second, 3*time.Second)
		err = resource.Retry(d.Timeout(schema.TimeoutUpdate), func() *resource.RetryError {
			response, err = client.RpcPost("Dts", "2020-01-01", action, nil, modifyDtsJobPasswordReq, false)
			if err != nil {
				if NeedRetry(err) {
					wait()
					return resource.RetryableError(err)
				}
				return resource.NonRetryableError(err)
			}
			return nil
		})
		addDebug(action, response, modifyDtsJobPasswordReq)
		if err != nil {
			return WrapErrorf(err, DefaultErrorMsg, d.Id(), action, AlibabaCloudSdkGoERROR)
		}
		if fmt.Sprint(response["Success"]) == "false" {
			return WrapError(fmt.Errorf("%s failed, response: %v", action, response))
		}
		d.SetPartial("destination_endpoint_password")
		d.SetPartial("destination_endpoint_user_name")

		target := d.Get("status").(string)
		err = resourceAlicloudDtsSynchronizationJobStatusFlow(d, meta, target)
		if err != nil {
			return WrapError(Error(FailedToReachTargetStatus, d.Get("status")))
		}
	}

	update = false
	request = map[string]interface{}{
		"DtsJobId": d.Id(),
	}
	if d.HasChange("instance_class") {
		if v, ok := d.GetOk("instance_class"); ok {
			request["InstanceClass"] = v
		}
		update = true
	}
	request["RegionId"] = client.RegionId
	request["OrderType"] = "UPGRADE"

	if update {
		action := "TransferInstanceClass"
		wait := incrementalWait(3*time.Second, 3*time.Second)
		err = resource.Retry(d.Timeout(schema.TimeoutUpdate), func() *resource.RetryError {
			response, err = client.RpcPost("Dts", "2020-01-01", action, nil, request, false)
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
			return WrapErrorf(err, DefaultErrorMsg, d.Id(), action, AlibabaCloudSdkGoERROR)
		}
		if fmt.Sprint(response["Success"]) == "false" {
			return WrapError(fmt.Errorf("%s failed, response: %v", action, response))
		}
	}

	if d.HasChange("status") {
		target := d.Get("status").(string)
		err := resourceAlicloudDtsSynchronizationJobStatusFlow(d, meta, target)
		if err != nil {
			return WrapError(err)
		}
	}

	modifyDtsJobReq := map[string]interface{}{
		"DtsInstanceId": d.Get("dts_instance_id"),
	}
	modifyDtsJobReq["RegionId"] = client.RegionId
	if !d.IsNewResource() && d.HasChange("db_list") {

		if v, ok := d.GetOk("db_list"); ok {
			modifyDtsJobReq["DbList"] = v
		}

		action := "ModifyDtsJob"
		wait := incrementalWait(3*time.Second, 3*time.Second)
		err = resource.Retry(d.Timeout(schema.TimeoutUpdate), func() *resource.RetryError {
			response, err = client.RpcPost("Dts", "2020-01-01", action, nil, modifyDtsJobReq, false)
			if err != nil {
				if IsExpectedErrors(err, []string{"InvalidJobStatus", "InvalidTaskStatus", "DTS.Msg.OperationDenied.JobStatusModifying", "DTS.Msg.ModifyDenied.JobStatusNotRunning"}) || NeedRetry(err) {
					wait()
					return resource.RetryableError(err)
				}
				return resource.NonRetryableError(err)
			}
			return nil
		})
		addDebug(action, response, modifyDtsJobReq)

		if err != nil {
			return WrapErrorf(err, DefaultErrorMsg, d.Id(), action, AlibabaCloudSdkGoERROR)
		}
		if fmt.Sprint(response["Success"]) == "false" {
			return WrapError(fmt.Errorf("%s failed, response: %v", action, response))
		}
		d.SetPartial("db_list")

		target := d.Get("status").(string)
		err = resourceAlicloudDtsSynchronizationJobStatusFlow(d, meta, target)
		if err != nil {
			return WrapError(Error(FailedToReachTargetStatus, d.Get("status")))
		}
	}
	d.Partial(false)
	return resourceAlicloudDtsSynchronizationJobRead(d, meta)
}
func resourceAlicloudDtsSynchronizationJobDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*connectivity.AliyunClient)
	action := "DeleteDtsJob"
	var response map[string]interface{}
	var err error
	request := map[string]interface{}{
		"DtsJobId": d.Id(),
	}

	if v, ok := d.GetOk("dts_instance_id"); ok {
		request["DtsInstanceId"] = v
	}
	request["RegionId"] = client.RegionId
	wait := incrementalWait(3*time.Second, 3*time.Second)
	err = resource.Retry(d.Timeout(schema.TimeoutDelete), func() *resource.RetryError {
		response, err = client.RpcPost("Dts", "2020-01-01", action, nil, request, false)
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
		if IsExpectedErrors(err, []string{"Forbidden.InstanceNotFound"}) {
			return nil
		}
		return WrapErrorf(err, DefaultErrorMsg, d.Id(), action, AlibabaCloudSdkGoERROR)
	}
	return nil
}

func resourceAlicloudDtsSynchronizationJobStatusFlow(d *schema.ResourceData, meta interface{}, target string) error {

	client := meta.(*connectivity.AliyunClient)
	dtsService := DtsService{client}
	var response map[string]interface{}
	var err error
	object, err := dtsService.DescribeDtsSynchronizationJob(d.Id())
	if err != nil {
		return WrapError(err)
	}
	if object["Status"].(string) != target {
		if target == "Synchronizing" || target == "Suspending" {
			request := map[string]interface{}{
				"DtsJobId": d.Id(),
			}
			request["RegionId"] = client.RegionId
			if v, ok := d.GetOk("synchronization_direction"); ok {
				request["SynchronizationDirection"] = v
			}
			action := "StartDtsJob"
			wait := incrementalWait(3*time.Second, 3*time.Second)
			err = resource.Retry(d.Timeout(schema.TimeoutUpdate), func() *resource.RetryError {
				response, err = client.RpcPost("Dts", "2020-01-01", action, nil, request, false)
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
				return WrapErrorf(err, DefaultErrorMsg, d.Id(), action, AlibabaCloudSdkGoERROR)
			}
			stateConf := BuildStateConf([]string{}, []string{"Synchronizing"}, d.Timeout(schema.TimeoutUpdate), 60*time.Second, dtsService.DtsSynchronizationJobStateRefreshFunc(d.Id(), []string{"PrecheckFailed", "InitializeFailed", "Failed"}))
			if _, err := stateConf.WaitForState(); err != nil {
				return WrapErrorf(err, IdMsg, d.Id())
			}
		}
		if target == "Suspending" {
			request := map[string]interface{}{
				"DtsJobId": d.Id(),
			}
			request["RegionId"] = client.RegionId
			if v, ok := d.GetOk("synchronization_direction"); ok {
				request["SynchronizationDirection"] = v
			}
			action := "SuspendDtsJob"
			wait := incrementalWait(3*time.Second, 3*time.Second)
			err = resource.Retry(d.Timeout(schema.TimeoutUpdate), func() *resource.RetryError {
				response, err = client.RpcPost("Dts", "2020-01-01", action, nil, request, false)
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
				return WrapErrorf(err, DefaultErrorMsg, d.Id(), action, AlibabaCloudSdkGoERROR)
			}
			stateConf := BuildStateConf([]string{}, []string{"Suspending"}, d.Timeout(schema.TimeoutUpdate), 5*time.Second, dtsService.DtsSynchronizationJobStateRefreshFunc(d.Id(), []string{"PrecheckFailed", "InitializeFailed", "Failed"}))
			if _, err := stateConf.WaitForState(); err != nil {
				return WrapErrorf(err, IdMsg, d.Id())
			}
		}
		d.SetPartial("status")
	}

	return nil
}

func convertSourceEndpointEngineNameUppercaseResponse(source interface{}) interface{} {
	switch source {
	case "polardb_pg":
		return "POLARDB_PG"
	case "express":
		return "EXPRESS"
	case "PostgreSQL":
		return "POSTGRESQL"
	case "MongoDB":
		return "MONGODB"
	case "as400":
		return "AS400"
	case "Redis":
		return "REDIS"
	case "TeraData":
		return "TERADATA"
	case "polardb_o":
		return "POLARDB_O"
	case "polardbx20":
		return "POLARDBX20"
	case "Oracle":
		return "ORACLE"
	case "DMSPolarDB":
		return "DMSPOLARDB"
	}
	return source
}
