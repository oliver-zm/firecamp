package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/golang/glog"

	"github.com/cloudstax/firecamp/common"
	"github.com/cloudstax/firecamp/db"
	"github.com/cloudstax/firecamp/db/awsdynamodb"
)

const (
	TablePartitionKey = "PartitionKey"
	TableSortKey      = "SortKey"

	devicePartitionKeyPrefix  = "DeviceKey-"
	servicePartitionKeyPrefix = "ServiceKey-"
)

func main() {
	flag.Parse()

	tableName := "test-table"
	config := aws.NewConfig().WithRegion("us-west-1")
	sess, err := session.NewSession(config)
	if err != nil {
		glog.Errorln("CreateServiceAttr failed to create session, error", err)
		os.Exit(-1)
	}

	dbIns := dynamodb.New(sess)

	err = createDeviceTable(dbIns, tableName)
	if err != nil {
		return
	}

	defer deleteTable(dbIns, tableName)
	time.Sleep(20 * time.Second)

	cluster := "cluster1"
	device := "device1"
	service := "service1"

	dev := &common.Device{
		ClusterName: cluster,
		DeviceName:  device,
		ServiceName: service,
	}
	createDevice(dbIns, tableName, dev)
	getDevice(dbIns, tableName, cluster, device)
	deleteDevice(dbIns, tableName, cluster, device, false)
	deleteDevice(dbIns, tableName, cluster, device, true)

	table2 := "test-table2"
	err = createTable(dbIns, table2)
	if err != nil {
		return
	}

	defer deleteTable(dbIns, table2)
	time.Sleep(20 * time.Second)

	shareCreateDevice(dbIns, table2, dev)
	shareGetDevice(dbIns, table2, cluster, device)
	shareDeleteDevice(dbIns, table2, cluster, device, false)
	shareDeleteDevice(dbIns, table2, cluster, device, true)
}

func createDeviceTable(dbIns *dynamodb.DynamoDB, tableName string) error {
	params := &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		AttributeDefinitions: []*dynamodb.AttributeDefinition{
			{
				AttributeName: aws.String(db.ClusterName),
				AttributeType: aws.String(dynamodb.ScalarAttributeTypeS),
			},
			{
				AttributeName: aws.String(db.DeviceName),
				AttributeType: aws.String(dynamodb.ScalarAttributeTypeS),
			},
		},
		KeySchema: []*dynamodb.KeySchemaElement{
			{
				AttributeName: aws.String(db.ClusterName),
				KeyType:       aws.String(dynamodb.KeyTypeHash),
			},
			{
				AttributeName: aws.String(db.DeviceName),
				KeyType:       aws.String(dynamodb.KeyTypeRange),
			},
		},
		ProvisionedThroughput: &dynamodb.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(1),
			WriteCapacityUnits: aws.Int64(1),
		},
	}
	resp, err := dbIns.CreateTable(params)

	if err != nil && err.(awserr.Error).Code() != "ResourceInUseException" {
		glog.Errorln("failed to create table", tableName, "error", err)
		return err
	}

	glog.Infoln("device table", tableName, "created or exists, resp", resp)
	return nil
}

func createDevice(dbsvc *dynamodb.DynamoDB, tableName string, dev *common.Device) error {
	params := &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item: map[string]*dynamodb.AttributeValue{
			db.ClusterName: {
				S: aws.String(dev.ClusterName),
			},
			db.DeviceName: {
				S: aws.String(dev.DeviceName),
			},
			db.ServiceName: {
				S: aws.String(dev.ServiceName),
			},
		},
		ConditionExpression:    aws.String(db.DevicePutCondition),
		ReturnConsumedCapacity: aws.String(dynamodb.ReturnConsumedCapacityTotal),
	}
	resp, err := dbsvc.PutItem(params)

	if err != nil {
		glog.Errorln("failed to create device", dev, "error", err)
		return awsdynamodb.ConvertError(err)
	}

	glog.Infoln("created device", dev, "resp", resp)
	return nil
}

func getDevice(svc *dynamodb.DynamoDB, tableName string, clusterName string, deviceName string) {
	params := &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]*dynamodb.AttributeValue{
			db.ClusterName: {
				S: aws.String(clusterName),
			},
			db.DeviceName: {
				S: aws.String(deviceName),
			},
		},
		ConsistentRead:         aws.Bool(true),
		ReturnConsumedCapacity: aws.String(dynamodb.ReturnConsumedCapacityTotal),
	}
	resp, err := svc.GetItem(params)
	fmt.Println("getDevice, cluster", clusterName, "device", deviceName, "resp", resp, "error", err)

	// access empty map will cause "panic: runtime error: invalid memory address or nil pointer dereference"
	// fmt.Println("access empty map", *(resp.Item[deviceName].S))
}

func deleteDevice(svc *dynamodb.DynamoDB, tableName string, clusterName string, deviceName string, setCond bool) {
	params := &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: map[string]*dynamodb.AttributeValue{
			db.ClusterName: {
				S: aws.String(clusterName),
			},
			db.DeviceName: {
				S: aws.String(deviceName),
			},
		},
		ReturnConsumedCapacity: aws.String(dynamodb.ReturnConsumedCapacityTotal),
	}
	if setCond {
		params.ConditionExpression = aws.String(db.DeviceDelCondition)
	}

	resp, err := svc.DeleteItem(params)
	fmt.Println("deleteDevice, cluster", clusterName, "device", deviceName, "setCond", setCond, "resp", resp, "error", err)
}

func createTable(dbIns *dynamodb.DynamoDB, tableName string) error {
	params := &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		AttributeDefinitions: []*dynamodb.AttributeDefinition{
			{
				AttributeName: aws.String(TablePartitionKey),
				AttributeType: aws.String(dynamodb.ScalarAttributeTypeS),
			},
			{
				AttributeName: aws.String(TableSortKey),
				AttributeType: aws.String(dynamodb.ScalarAttributeTypeS),
			},
		},
		KeySchema: []*dynamodb.KeySchemaElement{
			{
				AttributeName: aws.String(TablePartitionKey),
				KeyType:       aws.String(dynamodb.KeyTypeHash),
			},
			{
				AttributeName: aws.String(TableSortKey),
				KeyType:       aws.String(dynamodb.KeyTypeRange),
			},
		},
		ProvisionedThroughput: &dynamodb.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(1),
			WriteCapacityUnits: aws.Int64(1),
		},
	}
	resp, err := dbIns.CreateTable(params)

	if err != nil && err.(awserr.Error).Code() != "ResourceInUseException" {
		glog.Errorln("failed to create table", tableName, "error", err)
		return err
	}

	glog.Infoln("device table", tableName, "created or exists, resp", resp)
	return nil
}

func shareCreateDevice(dbsvc *dynamodb.DynamoDB, tableName string, dev *common.Device) error {
	params := &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item: map[string]*dynamodb.AttributeValue{
			TablePartitionKey: {
				S: aws.String(devicePartitionKeyPrefix + dev.ClusterName),
			},
			TableSortKey: {
				S: aws.String(dev.DeviceName),
			},
			db.ServiceName: {
				S: aws.String(dev.ServiceName),
			},
		},
		ConditionExpression:    aws.String("attribute_not_exists(" + TableSortKey + ")"),
		ReturnConsumedCapacity: aws.String(dynamodb.ReturnConsumedCapacityTotal),
	}
	resp, err := dbsvc.PutItem(params)

	if err != nil {
		glog.Errorln("failed to create device", dev, "error", err)
		return awsdynamodb.ConvertError(err)
	}

	glog.Infoln("created device", dev, "resp", resp)
	return nil
}

func shareGetDevice(svc *dynamodb.DynamoDB, tableName string, clusterName string, deviceName string) {
	params := &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]*dynamodb.AttributeValue{
			TablePartitionKey: {
				S: aws.String(devicePartitionKeyPrefix + clusterName),
			},
			TableSortKey: {
				S: aws.String(deviceName),
			},
		},
		ConsistentRead:         aws.Bool(true),
		ReturnConsumedCapacity: aws.String(dynamodb.ReturnConsumedCapacityTotal),
	}
	resp, err := svc.GetItem(params)
	fmt.Println("getDevice, cluster", clusterName, "device", deviceName, "resp", resp, "error", err)

	// access empty map will cause "panic: runtime error: invalid memory address or nil pointer dereference"
	// fmt.Println("access empty map", *(resp.Item[deviceName].S))
}

func shareDeleteDevice(svc *dynamodb.DynamoDB, tableName string, clusterName string, deviceName string, setCond bool) {
	params := &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: map[string]*dynamodb.AttributeValue{
			TablePartitionKey: {
				S: aws.String(devicePartitionKeyPrefix + clusterName),
			},
			TableSortKey: {
				S: aws.String(deviceName),
			},
		},
		ReturnConsumedCapacity: aws.String(dynamodb.ReturnConsumedCapacityTotal),
	}
	if setCond {
		params.ConditionExpression = aws.String("attribute_exists(" + TableSortKey + ")")
	}

	resp, err := svc.DeleteItem(params)
	fmt.Println("deleteDevice, cluster", clusterName, "device", deviceName, "setCond", setCond, "resp", resp, "error", err)
}

func deleteTable(svc *dynamodb.DynamoDB, tableName string) error {
	params := &dynamodb.DeleteTableInput{
		TableName: aws.String(tableName),
	}
	resp, err := svc.DeleteTable(params)

	if err != nil {
		glog.Errorln("failed to delete table", tableName, "error", err)
		return err
	}

	glog.Infoln("deleted table", tableName, "resp", resp)
	return nil
}
