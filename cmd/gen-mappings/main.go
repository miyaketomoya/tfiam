// gen-mappings generates internal/plan/mappings/aws.yaml from CloudFormation resource schemas.
//
// Usage:
//
//	go run ./cmd/gen-mappings --out internal/plan/mappings/aws.yaml
//	go run ./cmd/gen-mappings > internal/plan/mappings/aws.yaml
//
// Required AWS permissions: cloudformation:ListTypes, cloudformation:DescribeType
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"gopkg.in/yaml.v3"
)

// ── YAML output types (must match internal/plan/mappings/aws.yaml) ──────────

type ActionItem struct {
	Action     string `yaml:"action"`
	Confidence string `yaml:"confidence"`
	PassRole   bool   `yaml:"passrole,omitempty"`
}

type ResourceEntry struct {
	Create []ActionItem `yaml:"create,omitempty"`
	Update []ActionItem `yaml:"update,omitempty"`
	Delete []ActionItem `yaml:"delete,omitempty"`
}

// MappingFile mirrors the top-level shape: `resources: {aws_foo: ...}`.
// yaml.v3 marshals map keys in sorted order, so output is deterministic.
type MappingFile struct {
	Resources map[string]ResourceEntry `yaml:"resources"`
}

// ── CloudFormation schema types ──────────────────────────────────────────────

type cfnSchema struct {
	Handlers struct {
		Create *cfnHandler `json:"create"`
		Update *cfnHandler `json:"update"`
		Delete *cfnHandler `json:"delete"`
	} `json:"handlers"`
}

type cfnHandler struct {
	Permissions []string `json:"permissions"`
}

// ── Name conversion ──────────────────────────────────────────────────────────

// cfnToTF converts AWS::S3::Bucket → aws_s3_bucket.
// Diverging names are handled by the override table below.
func cfnToTF(cfnType string) string {
	if tf, ok := tfNameOverrides[cfnType]; ok {
		return tf
	}
	s := strings.TrimPrefix(cfnType, "AWS::")
	return "aws_" + strings.ToLower(strings.ReplaceAll(s, "::", "_"))
}

// tfNameOverrides maps CFn type names that don't follow the automatic pattern
// to the canonical Terraform resource type name.
var tfNameOverrides = map[string]string{
	// EC2 resources drop the "ec2_" prefix in Terraform
	"AWS::EC2::Instance":                         "aws_instance",
	"AWS::EC2::VPC":                              "aws_vpc",
	"AWS::EC2::Subnet":                           "aws_subnet",
	"AWS::EC2::SecurityGroup":                    "aws_security_group",
	"AWS::EC2::RouteTable":                       "aws_route_table",
	"AWS::EC2::InternetGateway":                  "aws_internet_gateway",
	"AWS::EC2::EIP":                              "aws_eip",
	"AWS::EC2::NatGateway":                       "aws_nat_gateway",
	"AWS::EC2::NetworkAcl":                       "aws_network_acl",
	"AWS::EC2::NetworkInterface":                 "aws_network_interface",
	"AWS::EC2::VPCEndpoint":                      "aws_vpc_endpoint",
	"AWS::EC2::VPCPeeringConnection":             "aws_vpc_peering_connection",
	"AWS::EC2::VPNGateway":                       "aws_vpn_gateway",
	"AWS::EC2::CustomerGateway":                  "aws_customer_gateway",
	"AWS::EC2::VPNConnection":                    "aws_vpn_connection",
	"AWS::EC2::TransitGateway":                   "aws_ec2_transit_gateway",
	"AWS::EC2::TransitGatewayAttachment":         "aws_ec2_transit_gateway_vpc_attachment",
	"AWS::EC2::LaunchTemplate":                   "aws_launch_template",
	"AWS::EC2::Fleet":                            "aws_ec2_fleet",
	"AWS::EC2::CapacityReservation":              "aws_ec2_capacity_reservation",
	// CloudWatch / Logs
	"AWS::Logs::LogGroup":             "aws_cloudwatch_log_group",
	"AWS::CloudWatch::Alarm":          "aws_cloudwatch_metric_alarm",
	"AWS::CloudWatch::Dashboard":      "aws_cloudwatch_dashboard",
	"AWS::Events::Rule":               "aws_cloudwatch_event_rule",
	"AWS::Events::EventBus":           "aws_cloudwatch_event_bus",
	// RDS
	"AWS::RDS::DBInstance": "aws_db_instance",
	"AWS::RDS::DBCluster":  "aws_rds_cluster",
	// ELB v2
	"AWS::ElasticLoadBalancingV2::LoadBalancer": "aws_lb",
	"AWS::ElasticLoadBalancingV2::TargetGroup":  "aws_lb_target_group",
	"AWS::ElasticLoadBalancingV2::Listener":     "aws_lb_listener",
	// ECS
	"AWS::ECS::TaskDefinition": "aws_ecs_task_definition",
	"AWS::ECS::Service":        "aws_ecs_service",
	"AWS::ECS::Cluster":        "aws_ecs_cluster",
	// AutoScaling
	"AWS::AutoScaling::AutoScalingGroup":           "aws_autoscaling_group",
	"AWS::AutoScaling::LaunchConfiguration":        "aws_launch_configuration",
	"AWS::AutoScaling::ScalingPolicy":              "aws_autoscaling_policy",
	"AWS::ApplicationAutoScaling::ScalableTarget":  "aws_appautoscaling_target",
	"AWS::ApplicationAutoScaling::ScalingPolicy":   "aws_appautoscaling_policy",
	// Lambda
	"AWS::Lambda::EventSourceMapping": "aws_lambda_event_source_mapping",
	"AWS::Lambda::LayerVersion":       "aws_lambda_layer_version",
	"AWS::Lambda::Permission":         "aws_lambda_permission",
	"AWS::Lambda::Alias":              "aws_lambda_alias",
	// ElastiCache
	"AWS::ElastiCache::ReplicationGroup": "aws_elasticache_replication_group",
	"AWS::ElastiCache::CacheCluster":     "aws_elasticache_cluster",
	// OpenSearch / Elasticsearch
	"AWS::Elasticsearch::Domain":   "aws_elasticsearch_domain",
	"AWS::OpenSearchService::Domain": "aws_opensearch_domain",
	// API Gateway
	"AWS::ApiGateway::RestApi": "aws_api_gateway_rest_api",
	"AWS::ApiGatewayV2::Api":   "aws_apigatewayv2_api",
	// Step Functions
	"AWS::StepFunctions::StateMachine": "aws_sfn_state_machine",
	// CodePipeline
	"AWS::CodePipeline::Pipeline": "aws_codepipeline",
	"AWS::CodeDeploy::Application":    "aws_codedeploy_app",
	"AWS::CodeDeploy::DeploymentGroup": "aws_codedeploy_deployment_group",
	"AWS::CodeCommit::Repository":     "aws_codecommit_repository",
	// Cognito
	"AWS::Cognito::UserPool":       "aws_cognito_user_pool",
	"AWS::Cognito::UserPoolClient": "aws_cognito_user_pool_client",
	// WAF
	"AWS::WAFv2::WebACL": "aws_wafv2_web_acl",
	// CloudFront
	"AWS::CloudFront::Distribution": "aws_cloudfront_distribution",
	// Route53
	"AWS::Route53::HostedZone": "aws_route53_zone",
	"AWS::Route53::RecordSet":  "aws_route53_record",
	// Secrets / SSM
	"AWS::SecretsManager::Secret": "aws_secretsmanager_secret",
	"AWS::SSM::Parameter":         "aws_ssm_parameter",
	// Kinesis
	"AWS::Kinesis::Stream":                  "aws_kinesis_stream",
	"AWS::KinesisFirehose::DeliveryStream":  "aws_kinesis_firehose_delivery_stream",
	// Glue
	"AWS::Glue::Database": "aws_glue_catalog_database",
	"AWS::Glue::Table":    "aws_glue_catalog_table",
	// EKS
	"AWS::EKS::Cluster":   "aws_eks_cluster",
	"AWS::EKS::Nodegroup": "aws_eks_node_group",
	// EFS
	"AWS::EFS::FileSystem":  "aws_efs_file_system",
	"AWS::EFS::MountTarget": "aws_efs_mount_target",
	// MSK
	"AWS::MSK::Cluster": "aws_msk_cluster",
	// MQ
	"AWS::MQ::Broker":       "aws_mq_broker",
	"AWS::AmazonMQ::Broker": "aws_mq_broker",
	// DAX
	"AWS::DAX::Cluster": "aws_dax_cluster",
	// Neptune
	"AWS::Neptune::DBCluster":  "aws_neptune_cluster",
	"AWS::Neptune::DBInstance": "aws_neptune_cluster_instance",
	// DocumentDB
	"AWS::DocDB::DBCluster":  "aws_docdb_cluster",
	"AWS::DocDB::DBInstance": "aws_docdb_cluster_instance",
	// IAM
	"AWS::IAM::InstanceProfile":  "aws_iam_instance_profile",
	"AWS::IAM::User":             "aws_iam_user",
	"AWS::IAM::Group":            "aws_iam_group",
	"AWS::IAM::OIDCProvider":     "aws_iam_openid_connect_provider",
	"AWS::IAM::SAMLProvider":     "aws_iam_saml_provider",
	"AWS::IAM::ServiceLinkedRole": "aws_iam_service_linked_role",
	// SNS
	"AWS::SNS::Subscription": "aws_sns_topic_subscription",
	// SQS
	"AWS::SQS::QueuePolicy": "aws_sqs_queue_policy",
	// S3
	"AWS::S3::BucketPolicy": "aws_s3_bucket_policy",
	// Misc
	"AWS::CloudTrail::Trail":               "aws_cloudtrail",
	"AWS::Config::ConfigRule":              "aws_config_config_rule",
	"AWS::GuardDuty::Detector":             "aws_guardduty_detector",
	"AWS::SecurityHub::Hub":                "aws_securityhub_account",
	"AWS::ACM::Certificate":                "aws_acm_certificate",
	"AWS::ACMPCA::CertificateAuthority":    "aws_acmpca_certificate_authority",
	"AWS::RAM::ResourceShare":              "aws_ram_resource_share",
	"AWS::AppSync::GraphQLApi":             "aws_appsync_graphql_api",
	"AWS::AppRunner::Service":              "aws_apprunner_service",
	"AWS::Redshift::Cluster":               "aws_redshift_cluster",
	"AWS::EMR::Cluster":                    "aws_emr_cluster",
	"AWS::Batch::JobDefinition":            "aws_batch_job_definition",
	"AWS::Batch::JobQueue":                 "aws_batch_job_queue",
	"AWS::Batch::ComputeEnvironment":       "aws_batch_compute_environment",
	"AWS::CloudFormation::Stack":           "aws_cloudformation_stack",
	"AWS::CloudFormation::StackSet":        "aws_cloudformation_stack_set",
	"AWS::Athena::WorkGroup":               "aws_athena_workgroup",
	"AWS::Transfer::Server":                "aws_transfer_server",
	"AWS::Lightsail::Instance":             "aws_lightsail_instance",
}

// ── Confidence heuristic ─────────────────────────────────────────────────────

// highVerbs are action verb prefixes that indicate the "core" operation for each handler type.
// Everything else (tagging, optional attribute writes) defaults to best-effort.
var highVerbs = map[string][]string{
	"create": {"Create", "Run", "Launch", "Allocate", "Register", "Deploy", "Start", "Provision"},
	"update": {"Update"},
	"delete": {"Delete", "Terminate", "Deregister", "Release", "Destroy", "Disable"},
}

var bestEffortPrefixes = []string{"Tag", "AddTags", "CreateTags", "RemoveTags", "Untag", "Label"}

func confidenceFor(handler, action string) string {
	if action == "iam:PassRole" {
		return "high"
	}
	parts := strings.SplitN(action, ":", 2)
	if len(parts) != 2 {
		return "best-effort"
	}
	verb := parts[1]
	for _, be := range bestEffortPrefixes {
		if strings.HasPrefix(verb, be) {
			return "best-effort"
		}
	}
	for _, hv := range highVerbs[handler] {
		if strings.HasPrefix(verb, hv) {
			return "high"
		}
	}
	return "best-effort"
}

// ── Main ─────────────────────────────────────────────────────────────────────

func main() {
	region := flag.String("region", "us-east-1", "AWS region for CloudFormation API")
	concurrency := flag.Int("concurrency", 3, "parallel DescribeType requests")
	rateMs := flag.Int("rate-ms", 300, "milliseconds to sleep between DescribeType calls (per goroutine)")
	outFile := flag.String("out", "", "output file path (default: stdout)")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(*region),
		config.WithRetryMaxAttempts(5),
		config.WithRetryMode(aws.RetryModeAdaptive),
	)
	if err != nil {
		log.Fatalf("load AWS config: %v", err)
	}
	cfn := cloudformation.NewFromConfig(cfg)

	log.Println("listing CloudFormation resource types...")
	typeNames, err := listAWSTypes(ctx, cfn)
	if err != nil {
		log.Fatalf("list types: %v", err)
	}
	log.Printf("found %d resource types — fetching schemas...", len(typeNames))

	type result struct {
		tfName string
		entry  ResourceEntry
	}

	resultsCh := make(chan result, len(typeNames))
	sem := make(chan struct{}, *concurrency)
	var wg sync.WaitGroup

	for _, typeName := range typeNames {
		wg.Add(1)
		go func(cfnType string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			time.Sleep(time.Duration(*rateMs) * time.Millisecond)
			entry, err := describeAndConvert(ctx, cfn, cfnType)
			if err != nil {
				log.Printf("SKIP %s: %v", cfnType, err)
				return
			}
			if entry == nil {
				return
			}
			resultsCh <- result{tfName: cfnToTF(cfnType), entry: *entry}
		}(typeName)
	}

	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	mapping := MappingFile{Resources: map[string]ResourceEntry{}}
	for r := range resultsCh {
		mapping.Resources[r.tfName] = r.entry
	}
	count := len(mapping.Resources)
	log.Printf("generated mappings for %d resource types", count)

	// Guard against silently writing an empty file when API calls all fail.
	// A healthy run should produce well over 100 entries.
	if count < 50 {
		log.Fatalf("too few resources generated (%d). CloudFormation API calls may have failed — check the SKIP log lines above.", count)
	}

	out := marshalMapping(mapping)
	if *outFile != "" {
		if err := os.WriteFile(*outFile, out, 0o644); err != nil {
			log.Fatalf("write %s: %v", *outFile, err)
		}
		log.Printf("wrote %s", *outFile)
	} else {
		os.Stdout.Write(out)
	}
}

func listAWSTypes(ctx context.Context, cfn *cloudformation.Client) ([]string, error) {
	var names []string
	var nextToken *string
	for {
		out, err := cfn.ListTypes(ctx, &cloudformation.ListTypesInput{
			Type:       cfntypes.RegistryTypeResource,
			Visibility: cfntypes.VisibilityPublic,
			Filters:    &cfntypes.TypeFilters{Category: cfntypes.CategoryAwsTypes},
			NextToken:  nextToken,
		})
		if err != nil {
			return nil, err
		}
		for _, t := range out.TypeSummaries {
			if t.TypeName != nil {
				names = append(names, *t.TypeName)
			}
		}
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}
	sort.Strings(names)
	return names, nil
}

func describeAndConvert(ctx context.Context, cfn *cloudformation.Client, cfnType string) (*ResourceEntry, error) {
	out, err := cfn.DescribeType(ctx, &cloudformation.DescribeTypeInput{
		Type:     cfntypes.RegistryTypeResource,
		TypeName: aws.String(cfnType),
	})
	if err != nil {
		return nil, err
	}
	if out.Schema == nil {
		return nil, nil
	}

	var schema cfnSchema
	if err := json.Unmarshal([]byte(*out.Schema), &schema); err != nil {
		return nil, fmt.Errorf("parse schema: %w", err)
	}

	entry := &ResourceEntry{
		Create: toActionItems("create", schema.Handlers.Create),
		Update: toActionItems("update", schema.Handlers.Update),
		Delete: toActionItems("delete", schema.Handlers.Delete),
	}
	if len(entry.Create) == 0 && len(entry.Update) == 0 && len(entry.Delete) == 0 {
		return nil, nil
	}
	return entry, nil
}

func toActionItems(handler string, h *cfnHandler) []ActionItem {
	if h == nil {
		return nil
	}
	seen := map[string]bool{}
	var items []ActionItem
	for _, p := range h.Permissions {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		item := ActionItem{
			Action:     p,
			Confidence: confidenceFor(handler, p),
			PassRole:   p == "iam:PassRole",
		}
		items = append(items, item)
	}
	// high-confidence actions first, then alphabetically within each tier
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Confidence != items[j].Confidence {
			return items[i].Confidence == "high"
		}
		return items[i].Action < items[j].Action
	})
	return items
}

func marshalMapping(mapping MappingFile) []byte {
	header := fmt.Sprintf(
		"# AWS Resource Type → Required IAM Actions Mapping\n"+
			"# Source: AWS CloudFormation Resource Provider Schemas\n"+
			"# Generated: %s\n"+
			"# Do not edit by hand — regenerate with: go run ./cmd/gen-mappings --out internal/plan/mappings/aws.yaml\n\n",
		time.Now().UTC().Format("2006-01-02"),
	)

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(mapping); err != nil {
		log.Fatalf("marshal yaml: %v", err)
	}
	return append([]byte(header), buf.Bytes()...)
}
