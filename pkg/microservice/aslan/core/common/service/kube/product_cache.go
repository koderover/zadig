package kube

import (
	"fmt"
	"sync"
	"time"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/util"
)

type productCacheItem struct {
	product  *commonmodels.Product
	expireAt time.Time
}

var (
	productCache    sync.Map
	productCacheTTL = 30 * time.Second
	productNow      = time.Now
	loadProductFunc = func(opt *commonrepo.ProductFindOptions) (*commonmodels.Product, error) {
		return commonrepo.NewProductColl().Find(opt)
	}
)

// GetProductWithCache 优先从内存缓存读取 product 元数据。
// 如果缓存未命中或已过期，则重新查库并刷新缓存。
// 返回值始终是深拷贝，避免调用方修改缓存中的原始对象。
func GetProductWithCache(opt *commonrepo.ProductFindOptions) (*commonmodels.Product, error) {
	if opt == nil {
		return nil, fmt.Errorf("product find options cannot be nil")
	}

	key := productCacheKey(opt)
	now := productNow()
	if cached, ok := productCache.Load(key); ok {
		item, ok := cached.(*productCacheItem)
		if ok && now.Before(item.expireAt) {
			return cloneProduct(item.product)
		}
		// 过期缓存立即删除，避免后续请求继续命中旧值。
		productCache.Delete(key)
	}

	product, err := loadProductFunc(opt)
	if err != nil || product == nil {
		return product, err
	}

	cachedProduct, err := cloneProduct(product)
	if err != nil {
		return nil, err
	}

	productCache.Store(key, &productCacheItem{
		product:  cachedProduct,
		expireAt: now.Add(productCacheTTL),
	})

	return cloneProduct(cachedProduct)
}

// productCacheKey 将 product 查询条件编码成缓存 key。
// 这里把 Production 一并纳入 key，避免不同环境类型读到错误缓存。
func productCacheKey(opt *commonrepo.ProductFindOptions) string {
	production := "nil"
	if opt != nil && opt.Production != nil {
		production = fmt.Sprintf("%t", *opt.Production)
	}

	return fmt.Sprintf("%s:%s:%s:%s", opt.Name, opt.EnvName, opt.Namespace, production)
}

// cloneProduct 返回 product 的深拷贝。
// 缓存层和业务层不共享同一个指针，避免调用方改值后污染缓存。
func cloneProduct(product *commonmodels.Product) (*commonmodels.Product, error) {
	if product == nil {
		return nil, nil
	}

	cloned := new(commonmodels.Product)
	if err := util.DeepCopy(cloned, product); err != nil {
		return nil, err
	}
	return cloned, nil
}

// InvalidateProductCache 主动删除指定 key 的缓存。
// 适合在环境更新、配置变更等写路径成功后调用，进一步缩短脏数据窗口。
// func InvalidateProductCache(opt *commonrepo.ProductFindOptions) {
// 	if opt == nil {
// 		return
// 	}
// 	productCache.Delete(productCacheKey(opt))
// }
