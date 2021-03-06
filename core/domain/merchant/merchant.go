/**
 * Copyright 2014 @ z3q.net.
 * name :
 * author : jarryliu
 * date : 2013-12-12 16:55
 * description :
 * history :
 */

package merchant

import (
	"errors"
	"fmt"
	"github.com/jsix/gof/db/orm"
	"go2o/core/domain/interface/enum"
	"go2o/core/domain/interface/member"
	"go2o/core/domain/interface/merchant"
	"go2o/core/domain/interface/merchant/shop"
	"go2o/core/domain/interface/merchant/user"
	"go2o/core/domain/interface/merchant/wholesaler"
	"go2o/core/domain/interface/valueobject"
	si "go2o/core/domain/merchant/shop"
	userImpl "go2o/core/domain/merchant/user"
	wsImpl "go2o/core/domain/merchant/wholesale"
	"go2o/core/domain/tmp"
	"go2o/core/infrastructure"
	"go2o/core/infrastructure/domain"
	"go2o/core/infrastructure/domain/util"
	"go2o/core/variable"
	"math"
	"strings"
	"time"
)

var _ merchant.IMerchantManager = new(merchantManagerImpl)

type merchantManagerImpl struct {
	rep     merchant.IMerchantRepo
	valRepo valueobject.IValueRepo
}

func NewMerchantManager(rep merchant.IMerchantRepo,
	valRepo valueobject.IValueRepo) merchant.IMerchantManager {
	return &merchantManagerImpl{
		rep:     rep,
		valRepo: valRepo,
	}
}

// 创建会员申请商户密钥
func (m *merchantManagerImpl) CreateSignUpToken(memberId int32) string {
	return m.rep.CreateSignUpToken(memberId)
}

// 根据商户申请密钥获取会员编号
func (m *merchantManagerImpl) GetMemberFromSignUpToken(token string) int32 {
	return m.rep.GetMemberFromSignUpToken(token)
}

// 删除会员的商户申请资料
func (m *merchantManagerImpl) RemoveSignUp(memberId int32) error {
	_, err := tmp.Db().GetOrm().Delete(merchant.MchSignUp{}, "member_id=?", memberId)
	return err
}
func (m *merchantManagerImpl) saveSignUpInfo(v *merchant.MchSignUp) (int32, error) {
	v.UpdateTime = time.Now().Unix()
	return orm.I32(orm.Save(tmp.Db().GetOrm(), v, int(v.Id)))
}

// 检查商户注册信息是否正确
func (m *merchantManagerImpl) checkSignUpInfo(v *merchant.MchSignUp) error {
	if v.MemberId <= 0 {
		return errors.New("会员不存在")
	}
	//todo: validate and check merchant name exists
	if v.MchName == "" {
		return merchant.ErrMissingMerchantName
	}
	if v.CompanyName == "" {
		return merchant.ErrMissingCompanyName
	}
	if v.CompanyNo == "" {
		return merchant.ErrMissingCompanyNo
	}
	if v.Address == "" {
		return merchant.ErrMissingAddress
	}
	if v.PersonName == "" {
		return merchant.ErrMissingPersonName
	}
	if v.PersonId == "" {
		return merchant.ErrMissingPersonId
	}
	if util.CheckChineseCardID(v.PersonId) != nil {
		return merchant.ErrPersonCardId
	}
	if v.CompanyImage == "" {
		return merchant.ErrMissingCompanyImage
	}
	if v.PersonImage == "" {
		return merchant.ErrMissingPersonImage
	}
	return nil
}

// 提交商户注册信息
func (m *merchantManagerImpl) CommitSignUpInfo(v *merchant.MchSignUp) (int32, error) {
	err := m.checkSignUpInfo(v)
	if err != nil {
		return 0, err
	}
	v.Reviewed = enum.ReviewAwaiting
	v.SubmitTime = time.Now().Unix()
	return m.saveSignUpInfo(v)
}

// 审核商户注册信息
func (m *merchantManagerImpl) ReviewMchSignUp(id int32, pass bool, remark string) error {
	var err error
	v := m.GetSignUpInfo(id)
	if v == nil {
		return merchant.ErrNoSuchSignUpInfo
	}
	if pass {
		v.Reviewed = enum.ReviewPass
		v.Remark = ""
		if err = m.createNewMerchant(v); err != nil {
			return err
		}
	} else {
		v.Reviewed = enum.ReviewReject
		v.Remark = remark
		if strings.TrimSpace(v.Remark) == "" {
			return merchant.ErrRequireRejectRemark
		}
	}
	_, err = m.saveSignUpInfo(v)
	return err
}

// 创建新商户
func (m *merchantManagerImpl) createNewMerchant(v *merchant.MchSignUp) error {
	unix := time.Now().Unix()
	mchVal := &merchant.Merchant{
		MemberId: v.MemberId,
		// 商户名称
		Name: v.MchName,
		// 是否自营
		SelfSales: 0,
		// 商户等级
		Level: 1,
		// 标志
		Logo: "",
		// 省
		Province: v.Province,
		// 市
		City: v.City,
		// 区
		District: v.District,
		// 是否启用
		Enabled: 1,
		// 过期时间
		ExpiresTime: time.Now().Add(time.Hour * time.Duration(24*365)).Unix(),
		// 注册时间
		JoinTime: unix,
		// 更新时间
		UpdateTime: unix,
		// 登录时间
		LoginTime: 0,
		// 最后登录时间
		LastLoginTime: 0,
	}
	mch := m.rep.CreateMerchant(mchVal)

	err := mch.SetValue(mchVal)
	if err != nil {
		return err
	}
	mchId, err := mch.Save()
	if err == nil {
		names := m.valRepo.GetAreaNames([]int32{v.Province, v.City, v.District})
		location := strings.Join(names, "")
		ev := &merchant.EnterpriseInfo{
			MerchantId:   mchId,
			Name:         v.CompanyName,
			CompanyNo:    v.CompanyNo,
			PersonName:   v.PersonName,
			PersonIdNo:   v.PersonId,
			PersonImage:  v.PersonImage,
			Tel:          v.Phone,
			Province:     v.Province,
			City:         v.City,
			District:     v.District,
			Location:     location,
			Address:      v.Address,
			CompanyImage: v.CompanyImage,
			AuthDoc:      v.AuthDoc,
			Reviewed:     v.Reviewed,
			ReviewTime:   unix,
			Remark:       "",
			UpdateTime:   unix,
		}
		_, err = mch.ProfileManager().SaveEnterpriseInfo(ev)
		if err == nil {
			mch.ProfileManager().ReviewEnterpriseInfo(true, "")
		}
	}
	return err
}

// 获取商户申请信息
func (m *merchantManagerImpl) GetSignUpInfo(id int32) *merchant.MchSignUp {
	v := merchant.MchSignUp{}
	if tmp.Db().GetOrm().Get(id, &v) != nil {
		return nil
	}
	return &v
}

// 获取会员申请的商户信息
func (m *merchantManagerImpl) GetSignUpInfoByMemberId(memberId int32) *merchant.MchSignUp {
	v := merchant.MchSignUp{}
	if tmp.Db().GetOrm().GetBy(&v, "member_id=?", memberId) != nil {
		return nil
	}
	return &v
}

// 获取会员关联的商户
func (m *merchantManagerImpl) GetMerchantByMemberId(memberId int32) merchant.IMerchant {
	v := merchant.Merchant{}
	if tmp.Db().GetOrm().GetBy(&v, "member_id=?", memberId) == nil {
		return m.rep.CreateMerchant(&v)
	}
	return nil
}

var _ merchant.IMerchant = new(merchantImpl)

type merchantImpl struct {
	_value           *merchant.Merchant
	_account         merchant.IAccount
	_wholesaler      wholesaler.IWholesaler
	_host            string
	_rep             merchant.IMerchantRepo
	_wsRepo          wholesaler.IWholesaleRepo
	_shopRepo        shop.IShopRepo
	_userRepo        user.IUserRepo
	_valRepo         valueobject.IValueRepo
	_memberRepo      member.IMemberRepo
	_userManager     user.IUserManager
	_confManager     merchant.IConfManager
	_levelManager    merchant.ILevelManager
	_kvManager       merchant.IKvManager
	_memberKvManager merchant.IKvManager
	//_mssManager      mss.IMssManager
	//_mssRepo          mss.IMssRepo
	_profileManager merchant.IProfileManager
	_apiManager     merchant.IApiManager
	_shopManager    shop.IShopManager
}

func NewMerchant(v *merchant.Merchant, rep merchant.IMerchantRepo,
	wsRepo wholesaler.IWholesaleRepo, shopRepo shop.IShopRepo,
	userRepo user.IUserRepo, memberRepo member.IMemberRepo,
	valRepo valueobject.IValueRepo) merchant.IMerchant {
	mch := &merchantImpl{
		_value:      v,
		_rep:        rep,
		_wsRepo:     wsRepo,
		_shopRepo:   shopRepo,
		_userRepo:   userRepo,
		_valRepo:    valRepo,
		_memberRepo: memberRepo,
	}
	return mch
}

func (m *merchantImpl) GetRepo() merchant.IMerchantRepo {
	return m._rep
}

func (m *merchantImpl) GetAggregateRootId() int32 {
	return m._value.Id
}
func (m *merchantImpl) GetValue() merchant.Merchant {
	return *m._value
}

func (m *merchantImpl) SetValue(v *merchant.Merchant) error {
	tv := m._value
	if v.Id == tv.Id {
		tv.Name = v.Name
		tv.Province = v.Province
		tv.City = v.City
		tv.District = v.District
		if v.LastLoginTime > 0 {
			tv.LastLoginTime = v.LastLoginTime
		}
		if v.LoginTime > 0 {
			tv.LoginTime = v.LoginTime
		}

		if len(v.Logo) != 0 {
			tv.Logo = v.Logo
		}
		tv.Pwd = v.Pwd
		tv.UpdateTime = time.Now().Unix()
	}
	return nil
}

// 保存
func (m *merchantImpl) Save() (int32, error) {
	id := m.GetAggregateRootId()
	if id > 0 {
		m.checkSelfSales()
		return m._rep.SaveMerchant(m._value)
	}
	return m.createMerchant()
}

// 自营检测,并返回是否需要保存
func (m *merchantImpl) checkSelfSales() bool {
	if m._value.SelfSales == 0 {
		//不为自营,但编号为1的商户
		if m.GetAggregateRootId() == 1 {
			m._value.SelfSales = 1
			m._value.Usr = "-"
			m._value.Pwd = "-"
			return true
		}
	} else if m.GetAggregateRootId() != 1 {
		//为自营,但ID不为0,异常数据
		m._value.SelfSales = 0
		m._value.Enabled = 0
		return true
	}
	return false
}

// 是否自营
func (m *merchantImpl) SelfSales() bool {
	return m._value.SelfSales == 1 ||
		m.GetAggregateRootId() == 1
}

// 获取商户的状态,判断 过期时间、判断是否停用。
// 过期时间通常按: 试合作期,即1个月, 后面每年延长一次。或者会员付费开通。
func (m *merchantImpl) Stat() error {
	if m._value == nil {
		return merchant.ErrNoSuchMerchant
	}
	if m._value.Enabled == 0 {
		return merchant.ErrMerchantDisabled
	}
	if m._value.ExpiresTime < time.Now().Unix() {
		return merchant.ErrMerchantExpires
	}
	return nil
}

// 设置商户启用或停用
func (m *merchantImpl) SetEnabled(enabled bool) error {
	if enabled {
		m._value.Enabled = 1
	} else {
		m._value.Enabled = 0
	}
	_, err := m.Save()
	return err
}

// 返回对应的会员编号
func (m *merchantImpl) Member() int32 {
	return m.GetValue().MemberId
}

// 获取商户账户
func (m *merchantImpl) Account() merchant.IAccount {
	if m._account == nil {
		v := m._rep.GetAccount(m.GetAggregateRootId())
		m._account = newAccountImpl(m, v, m._memberRepo)
	}
	return m._account
}

// 获取批发商实例
func (m *merchantImpl) Wholesaler() wholesaler.IWholesaler {
	if m._wholesaler == nil {
		mchId := m.GetAggregateRootId()
		v := m._wsRepo.GetWsWholesaler(mchId)
		if v != nil {
			m._wholesaler = wsImpl.NewWholesaler(mchId, v, m._wsRepo)
		}
	}
	return m._wholesaler
}

// 启用批发
func (m *merchantImpl) EnableWholesale() error {
	if m.Wholesaler() != nil {
		return errors.New("wholesale for merchant enabled!")
	}
	v := &wholesaler.WsWholesaler{
		MchId:       m.GetAggregateRootId(),
		Rate:        1,
		ReviewState: enum.ReviewAwaiting,
	}
	_, err := m._wsRepo.SaveWsWholesaler(v, true)
	return err
}

// 创建商户
func (m *merchantImpl) createMerchant() (int32, error) {
	if id := m.GetAggregateRootId(); id > 0 {
		return id, nil
	}

	id, err := m._rep.SaveMerchant(m._value)
	if err != nil {
		return id, err
	}

	//todo:事务

	// 初始化商户信息
	m._value.Id = id

	// 检测自营并保存
	if m.checkSelfSales() {
		m._rep.SaveMerchant(m._value)
	}

	//todo:  初始化商店

	// SiteConf
	//m._siteConf = &shop.ShopSiteConf{
	//	IndexTitle: "线上商店-" + v.Name,
	//	SubTitle:   "线上商店-" + v.Name,
	//	Logo:       v.Logo,
	//	State:      1,
	//	StateHtml:  "",
	//}
	//err = m._rep.SaveSiteConf(id, m._siteConf)
	//m._siteConf.MerchantId = id

	// SaleConf
	//m._saleConf = &merchant.SaleConf{
	//	AutoSetupOrder:  1,
	//	IntegralBackNum: 0,
	//}
	//err = m._rep.SaveSaleConf(id, m._saleConf)
	//m._saleConf.MerchantId = id

	// 创建API
	api := &merchant.ApiInfo{
		ApiId:     domain.NewApiId(int(id)),
		ApiSecret: domain.NewSecret(int(id)),
		WhiteList: "*",
		Enabled:   1,
	}
	err = m.ApiManager().SaveApiInfo(api)
	return id, err
}

// 获取商户的域名
func (m *merchantImpl) GetMajorHost() string {
	if len(m._host) == 0 {
		host := m._rep.GetMerchantMajorHost(m.GetAggregateRootId())
		if len(host) == 0 {
			host = fmt.Sprintf("%s.%s", m._value.Usr, infrastructure.GetApp().
				Config().GetString(variable.ServerDomain))
		}
		m._host = host
	}
	return m._host
}

// 返回用户服务
func (m *merchantImpl) UserManager() user.IUserManager {
	if m._userManager == nil {
		m._userManager = userImpl.NewUserManager(
			m.GetAggregateRootId(),
			m._userRepo)
	}
	return m._userManager
}

// 获取会员管理服务
func (m *merchantImpl) LevelManager() merchant.ILevelManager {
	if m._levelManager == nil {
		m._levelManager = NewLevelManager(m.GetAggregateRootId(), m._rep)
	}
	return m._levelManager
}

// 获取键值管理器
func (m *merchantImpl) KvManager() merchant.IKvManager {
	if m._kvManager == nil {
		m._kvManager = newKvManager(m, "kvset")
	}
	return m._kvManager
}

// 获取用户键值管理器
func (m *merchantImpl) MemberKvManager() merchant.IKvManager {
	if m._memberKvManager == nil {
		m._memberKvManager = newKvManager(m, "kvset_member")
	}
	return m._memberKvManager
}

// 消息系统管理器
//func (m *MerchantImpl) MssManager() mss.IMssManager {
//	if m._mssManager == nil {
//		m._mssManager = mssImpl.NewMssManager(m, m._mssRepo, m._rep)
//	}
//	return m._mssManager
//}

// 返回设置服务
func (m *merchantImpl) ConfManager() merchant.IConfManager {
	if m._confManager == nil {
		m._confManager = newConfigManagerImpl(m.GetAggregateRootId(),
			m._rep, m._memberRepo, m._valRepo)
	}
	return m._confManager
}

// 企业资料管理器
func (m *merchantImpl) ProfileManager() merchant.IProfileManager {
	if m._profileManager == nil {
		m._profileManager = newProfileManager(m)
	}
	return m._profileManager
}

// API服务
func (m *merchantImpl) ApiManager() merchant.IApiManager {
	if m._apiManager == nil {
		m._apiManager = newApiManagerImpl(m)
	}
	return m._apiManager
}

// 商店服务
func (m *merchantImpl) ShopManager() shop.IShopManager {
	if m._shopManager == nil {
		m._shopManager = si.NewShopManagerImpl(m, m._shopRepo, m._valRepo)
	}
	return m._shopManager
}

var _ merchant.IAccount = new(accountImpl)

type accountImpl struct {
	mchImpl    *merchantImpl
	value      *merchant.Account
	memberRepo member.IMemberRepo
}

func newAccountImpl(mchImpl *merchantImpl, a *merchant.Account,
	memberRepo member.IMemberRepo) merchant.IAccount {
	return &accountImpl{
		mchImpl:    mchImpl,
		value:      a,
		memberRepo: memberRepo,
	}
}

// 获取领域对象编号
func (a *accountImpl) GetDomainId() int32 {
	return a.value.MchId
}

// 获取账户值
func (a *accountImpl) GetValue() *merchant.Account {
	return a.value
}

// 保存
func (a *accountImpl) Save() error {
	_, err := orm.Save(tmp.Db().GetOrm(), a.value, int(a.GetDomainId()))
	//_, err := a.mchImpl._rep.SaveMerchantAccount(a)
	return err
}

// 根据编号获取余额变动信息
func (a *accountImpl) GetBalanceLog(id int32) *merchant.BalanceLog {
	e := merchant.BalanceLog{}
	if tmp.Db().GetOrm().Get(id, &e) == nil {
		return &e
	}
	return nil
	//return a.mchImpl._rep.GetBalanceLog(id)
}

// 根据号码获取余额变动信息
func (a *accountImpl) GetBalanceLogByOuterNo(outerNo string) *merchant.BalanceLog {
	e := merchant.BalanceLog{}
	if tmp.Db().GetOrm().GetBy(&e, "outer_no=?", outerNo) == nil {
		return &e
	}
	return nil
	//return a.mchImpl._rep.GetBalanceLogByOuterNo(outerNo)
}

func (a *accountImpl) createBalanceLog(kind int, title string, outerNo string,
	amount float32, csn float32, state int) *merchant.BalanceLog {
	unix := time.Now().Unix()
	return &merchant.BalanceLog{
		// 编号
		Id: 0,
		// 商户编号
		MchId: a.GetDomainId(),
		// 日志类型
		Kind: kind,
		// 标题
		Title: title,
		// 外部订单号
		OuterNo: outerNo,
		// 金额
		Amount: amount,
		// 手续费
		CsnAmount: csn,
		// 状态
		State: state,
		// 创建时间
		CreateTime: unix,
		// 更新时间
		UpdateTime: unix,
	}
}

// 保存余额变动信息
func (a *accountImpl) SaveBalanceLog(v *merchant.BalanceLog) (int32, error) {
	return orm.I32(orm.Save(tmp.Db().GetOrm(), v, int(v.Id)))
	//return a.mchImpl._rep.SaveBalanceLog(v)
}

// 订单结账
func (a *accountImpl) SettleOrder(orderNo string, amount float32,
	csn float32, refundAmount float32, remark string) error {
	if amount <= 0 || math.IsNaN(float64(amount)) {
		return merchant.ErrAmount
	}
	l := a.createBalanceLog(merchant.KindAccountSettleOrder,
		remark, orderNo, amount, csn, 1)
	_, err := a.SaveBalanceLog(l)
	if err == nil {
		// 扣款
		a.value.Balance += amount
		a.value.SalesAmount += amount
		a.value.RefundAmount += refundAmount
		a.value.UpdateTime = time.Now().Unix()
		err = a.Save()
	}
	return err
}

//todo: 转入到奖金，手续费又被用于消费。这是一个bug

// 提现
//todo:???

// 转到会员账户
func (a *accountImpl) TransferToMember(amount float32) error {
	if amount <= 0 || math.IsNaN(float64(amount)) {
		return merchant.ErrAmount
	}
	if a.value.Balance < amount || a.value.Balance <= 0 {
		return merchant.ErrNoMoreAmount
	}
	if a.mchImpl._value.MemberId <= 0 {
		return member.ErrNoSuchMember
	}
	m := a.memberRepo.GetMember(a.mchImpl._value.MemberId)
	if m == nil {
		return member.ErrNoSuchMember
	}
	l := a.createBalanceLog(merchant.KindAccountTransferToMember,
		"提取到会员"+variable.AliasWalletAccount, "", -amount, 0, 1)
	_, err := a.SaveBalanceLog(l)
	if err == nil {
		err = m.GetAccount().Charge(member.AccountWallet,
			member.KindWalletAdd,
			variable.AliasMerchantBalanceAccount+
				"提现", "", amount, member.DefaultRelateUser)
		if err != nil {
			return err
		}
		a.value.Balance -= amount
		a.value.TakeAmount += amount
		a.value.UpdateTime = time.Now().Unix()
		err = a.Save()
		if err != nil {
			return err
		}

		// 判断是否提现免费,如果免费,则赠送手续费
		registry := a.mchImpl._valRepo.GetRegistry()
		if registry.MerchantTakeOutCashFree {
			conf := a.mchImpl._valRepo.GetGlobNumberConf()
			if conf.TakeOutCsn > 0 {
				csn := amount * conf.TakeOutCsn
				err = m.GetAccount().Charge(member.AccountWallet,
					member.KindWalletAdd, "返还商户提现手续费", "",
					csn, member.DefaultRelateUser)
			}
		}
	}

	return err
}

func (a *accountImpl) TransferToMember1(amount float32) error {
	if amount <= 0 || math.IsNaN(float64(amount)) {
		return merchant.ErrAmount
	}
	if a.value.Balance < amount || a.value.Balance <= 0 {
		return merchant.ErrNoMoreAmount
	}
	if a.mchImpl._value.MemberId <= 0 {
		return member.ErrNoSuchMember
	}
	m := a.memberRepo.GetMember(a.mchImpl._value.MemberId)
	if m == nil {
		return member.ErrNoSuchMember
	}
	l := a.createBalanceLog(merchant.KindAccountTransferToMember,
		"提取到会员"+variable.AliasWalletAccount, "", -amount, 0, 1)
	_, err := a.SaveBalanceLog(l)
	if err == nil {
		err = m.GetAccount().Charge(member.AccountWallet,
			member.KindWalletAdd, variable.AliasMerchantBalanceAccount+
				"提现", "", amount, member.DefaultRelateUser)
		if err != nil {
			return err
		}
		a.value.Balance -= amount
		a.value.TakeAmount += amount
		a.value.UpdateTime = time.Now().Unix()
		err = a.Save()
		if err != nil {
			return err
		}

		// 判断是否提现免费,如果免费,则赠送手续费
		registry := a.mchImpl._valRepo.GetRegistry()
		if registry.MerchantTakeOutCashFree {
			conf := a.mchImpl._valRepo.GetGlobNumberConf()
			if conf.TakeOutCsn > 0 {
				csn := float32(0)
				err = m.GetAccount().Charge(member.AccountWallet,
					member.KindWalletAdd, "返还商户提现手续费", "",
					csn, member.DefaultRelateUser)
			}
		}
	}

	return err
}

// 赠送
func (a *accountImpl) Present(amount float32, remark string) error {
	if amount <= 0 || math.IsNaN(float64(amount)) {
		return merchant.ErrAmount
	}
	l := a.createBalanceLog(merchant.KindAccountPresent,
		remark, "", amount, 0, 1)
	_, err := a.SaveBalanceLog(l)
	if err == nil {
		a.value.PresentAmount += amount
		a.value.UpdateTime = time.Now().Unix()
		err = a.Save()
	}
	return err
}
