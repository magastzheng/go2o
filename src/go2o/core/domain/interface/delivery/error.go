/**
 * Copyright 2014 @ ops Inc.
 * name :
 * author : newmin
 * date : 2013-12-12 16:53
 * description :
 * history :
 */

package delivery

import (
    "go2o/core/infrastructure/domain"
)

var (
    ErrNotCoveragedArea *domain.DomainError = domain.NewDomainError(
    "not_coveraged_area", "未覆盖的配送区域")
)


