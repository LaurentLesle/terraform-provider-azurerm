package sentinel

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/preview/securityinsight/mgmt/2022-01-01-preview/securityinsight"
	"github.com/hashicorp/go-azure-sdk/resource-manager/operationalinsights/2020-08-01/workspaces"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/sentinel/parse"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/sentinel/validate"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/validation"
	"github.com/hashicorp/terraform-provider-azurerm/utils"
)

type DataConnectorOfficePowerBIResource struct{}

var _ sdk.ResourceWithCustomImporter = DataConnectorOfficePowerBIResource{}

type DataConnectorOfficePowerBIModel struct {
	Name                    string `tfschema:"name"`
	LogAnalyticsWorkspaceId string `tfschema:"log_analytics_workspace_id"`
	TenantId                string `tfschema:"tenant_id"`
}

func (r DataConnectorOfficePowerBIResource) Arguments() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"name": {
			Type:         pluginsdk.TypeString,
			Required:     true,
			ForceNew:     true,
			ValidateFunc: validation.StringIsNotEmpty,
		},

		"log_analytics_workspace_id": {
			Type:         pluginsdk.TypeString,
			Required:     true,
			ForceNew:     true,
			ValidateFunc: workspaces.ValidateWorkspaceID,
		},

		"tenant_id": {
			Type:         pluginsdk.TypeString,
			Optional:     true,
			Computed:     true,
			ForceNew:     true,
			ValidateFunc: validation.IsUUID,
		},
	}
}

func (r DataConnectorOfficePowerBIResource) Attributes() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{}
}

func (r DataConnectorOfficePowerBIResource) ResourceType() string {
	return "azurerm_sentinel_data_connector_office_power_bi"
}

func (r DataConnectorOfficePowerBIResource) ModelObject() interface{} {
	return &DataConnectorOfficePowerBIModel{}
}

func (r DataConnectorOfficePowerBIResource) IDValidationFunc() pluginsdk.SchemaValidateFunc {
	return validate.DataConnectorID
}

func (r DataConnectorOfficePowerBIResource) CustomImporter() sdk.ResourceRunFunc {
	return func(ctx context.Context, metadata sdk.ResourceMetaData) error {
		_, err := importSentinelDataConnector(securityinsight.DataConnectorKindOfficePowerBI)(ctx, metadata.ResourceData, metadata.Client)
		return err
	}
}

func (r DataConnectorOfficePowerBIResource) Create() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 30 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.Sentinel.DataConnectorsClient

			var plan DataConnectorOfficePowerBIModel
			if err := metadata.Decode(&plan); err != nil {
				return fmt.Errorf("decoding %+v", err)
			}

			workspaceId, err := workspaces.ParseWorkspaceID(plan.LogAnalyticsWorkspaceId)
			if err != nil {
				return err
			}

			id := parse.NewDataConnectorID(workspaceId.SubscriptionId, workspaceId.ResourceGroupName, workspaceId.WorkspaceName, plan.Name)
			existing, err := client.Get(ctx, id.ResourceGroup, id.WorkspaceName, id.Name)
			if err != nil {
				if !utils.ResponseWasNotFound(existing.Response) {
					return fmt.Errorf("checking for presence of existing %s: %+v", id, err)
				}
			}
			if !utils.ResponseWasNotFound(existing.Response) {
				return metadata.ResourceRequiresImport(r.ResourceType(), id)
			}

			tenantId := plan.TenantId
			if tenantId == "" {
				tenantId = metadata.Client.Account.TenantId
			}

			params := securityinsight.OfficePowerBIDataConnector{
				Name: &plan.Name,
				OfficePowerBIDataConnectorProperties: &securityinsight.OfficePowerBIDataConnectorProperties{
					TenantID: &tenantId,
					DataTypes: &securityinsight.OfficePowerBIConnectorDataTypes{
						Logs: &securityinsight.OfficePowerBIConnectorDataTypesLogs{
							State: securityinsight.DataTypeStateEnabled,
						},
					},
				},
				Kind: securityinsight.KindBasicDataConnectorKindOfficePowerBI,
			}

			if _, err = client.CreateOrUpdate(ctx, id.ResourceGroup, id.WorkspaceName, id.Name, params); err != nil {
				return fmt.Errorf("creating %s: %+v", id, err)
			}

			metadata.SetID(id)
			return nil
		},
	}
}

func (r DataConnectorOfficePowerBIResource) Read() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 5 * time.Minute,

		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.Sentinel.DataConnectorsClient
			id, err := parse.DataConnectorID(metadata.ResourceData.Id())
			if err != nil {
				return err
			}

			workspaceId := workspaces.NewWorkspaceID(id.SubscriptionId, id.ResourceGroup, id.WorkspaceName)

			existing, err := client.Get(ctx, id.ResourceGroup, id.WorkspaceName, id.Name)
			if err != nil {
				if utils.ResponseWasNotFound(existing.Response) {
					return metadata.MarkAsGone(id)
				}
				return fmt.Errorf("retrieving %s: %+v", id, err)
			}

			dc, ok := existing.Value.(securityinsight.OfficePowerBIDataConnector)
			if !ok {
				return fmt.Errorf("%s was not an Office PowerBI Data Connector", id)
			}

			var tenantId string
			if props := dc.OfficePowerBIDataConnectorProperties; props != nil {
				if props.TenantID != nil {
					tenantId = *props.TenantID
				}
			}

			model := DataConnectorOfficePowerBIModel{
				Name:                    id.Name,
				LogAnalyticsWorkspaceId: workspaceId.ID(),
				TenantId:                tenantId,
			}

			return metadata.Encode(&model)
		},
	}
}

func (r DataConnectorOfficePowerBIResource) Delete() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 30 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.Sentinel.DataConnectorsClient

			id, err := parse.DataConnectorID(metadata.ResourceData.Id())
			if err != nil {
				return err
			}

			if _, err := client.Delete(ctx, id.ResourceGroup, id.WorkspaceName, id.Name); err != nil {
				return fmt.Errorf("deleting %s: %+v", id, err)
			}

			return nil
		},
	}
}
