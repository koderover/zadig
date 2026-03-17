package service

import "fmt"

const updateMultipleProductLockKeyFormat = "update_multiple_product:%s"

func updateMultipleProductLockKey(productName string) string {
	return fmt.Sprintf(updateMultipleProductLockKeyFormat, productName)
}

func serviceNameTypeKey(serviceName, serviceType string) string {
	return serviceName + ":" + serviceType
}
