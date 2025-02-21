package model

import "github.com/jackc/pgtype"

type User_id struct {
	User_id int `json:"user_id"`
}
type Owner_id struct {
	Owner_id int `json:"owner_id"`
}

type Ads_id struct { // используется в PrintAdsPOST, SigChatPOST, GroupReviewNewOnesFirstPOST, GroupReviewOldOnesFirstPOST, GroupReviewLowRatOnesFirstPOST, GroupReviewHigRatOnesFirstPOST
	Ads_id int `json:"ads_id"`
}

type Chat struct { // используетяс в SigDisputInChatPOST
	Id_chat int `json:"Id_chat"`
}

type LegalUser struct { // используется SignupLegalEmailPOST
	Avatar string `json:"avatar"`

	Password_hash string `json:"password_hash"`
	Email         string `json:"email" db:"email"`
	Phone_number  string `json:"phone_number"`

	Ind_num_taxp    int    `json:"ind_num_taxp"`
	Name_of_company string `json:"name_of_company"`
	Address_name    string `json:"address_name"`

	Filename string `json:"filename"`
	Filetype string `json:"filetype"`

	Data string `json:"data"`
}

type NaturUser struct {
	Avatar string `json:"avatar"`

	Password_hash string `json:"password_hash"`
	Email         string `json:"email" db:"email"`
	Phone_number  string `json:"phone_number"`

	Surname    string `json:"surname"`
	Name       string `json:"name"`
	Patronymic string `json:"patronymic"`

	Filename string `json:"filename"`
	Filetype string `json:"filetype"`
	Data     string `json:"data"`
}

type Reg_code struct { // используется в EnterCodeForRecoveryPassWithEmailPOST
	Reg_code int `json:"reg_code"`
}

type SendMessAndImg struct {
	Id_chat int      `json:"Id_chat"`
	Text    string   `json:"Text"`
	Images  []string `json:"Image"`
}

type SendImg struct {
	Id_chat int      `json:"Id_chat"`
	Images  []string `json:"Image"`
}

type SendMess struct {
	Id_chat int    `json:"Id_chat"`
	Text    string `json:"Text"`
}

type SendMessAndVideo struct {
	Id_chat int      `json:"Id_chat"`
	Text    string   `json:"Text"`
	Videos  []string `json:"Videos"`
}

type SendVideo struct {
	Id_chat int      `json:"Id_chat"`
	Videos  []string `json:"Videos"`
}

type Transact struct { // используется в процедуре TransactionToAnotherPOST
	User_2 int `json:"User_2"` // друг
	Amount int `json:"Amount"` // цена
}

type Email struct { // используется в RecoveryPasswdPOST, RecoveryPasswdPOST
	Email string `json:"Email"`
}

type Phone_kesh struct { // используется в EnterCodeFromPhonePOST
	Phone string `json:"Phone_num"`
	Code  int    `json:"Code"`
}

type Email_kesh struct {
	Email string `json:"Email"`
	Code  int    `json:"Code"`
}

type Kesh_passwd_code struct {
	Passwd string
	Code   int
}

type Email_name struct { // используется в SendCodForEmailPOST
	Email_name string `json:"email_name"`
}

type Phone_num struct { // используется в SendCodForPhoneNumPOST
	Phone_num string `json:"phone_num"`
}

type Response_SendCodForPhoneNum struct {
	Status  string `json:"status"`
	Data    Data   `json:"data,omitempty"`
	Message string `json:"message"`
}

type Data struct { // используется в SendCodForPhoneNumPOST
	CodeNum        int    `json:"CodeNum"`
	Phone_num      string `json:"Phone_num"`
	ValidToken_jwt string `json:"ValidToken_jwt"`
}

type Login struct { // используется в LoginPOST
	Login    string `json:"login"`
	Password string `json:"password"`
}

type ProductList struct { // используется в ProductListPOST
	Ads_list []int `json:"Ads_list"`
}

type SortProductListAll struct { // используется в SortProductListDailyRatePOST, SortProductListHourlyRatePOST, SortProductListCategoriezPOST
	List     int       `json:"List"`
	Size     int       `json:"Size"`
	Category []int     `json:"Category"`
	LowNum   int       `json:"LowNum"`
	HigNum   int       `json:"HigNum"`
	LowDate  int64     `json:"LowDate"`
	HigDate  int64     `json:"HigDate"`
	Position []float64 `json:"Position"`
	Location string    `json:"Location"`
	Distance int       `json:"Distance"`
	Rating   int       `json:"Rating"`
}

type Ads struct { // используется в SignupAdsPOST, SearchForTechPOST
	Image []string `json:"Image"`

	Id          int     `json:"Id"`
	Title       string  `json:"Title" validate:"required"`
	Description string  `json:"Description" validate:"required"`
	Hourly_rate int     `json:"Hourly_rate"`
	Daily_rate  int     `json:"Daily_rate"`
	Category_id int     `json:"Category_id" validate:"required"`
	PositionX   float64 `json:"PositionX"`
	PositionY   float64 `json:"PositionY"`
	Location    string  `json:"Location" validate:"required"`
}

type Upd_ads struct { // используется в UpdAdsPOST
	Ads_id             int          `json:"Ads_id"`
	Id_images_from_del []int        `json:"Images_from_del"`
	Title              string       `json:"Title"`
	Description        string       `json:"Description"`
	Hourly_rate        int          `json:"Hourly_rate"`
	Daily_rate         int          `json:"Daily_rate"`
	Category_id        int          `json:"Category_id"`
	Position           pgtype.Point `json:"Position"`
	Images             []string     `json:"Images"`
}

type FavAds struct { // используется в SigFavAdsPOST, DelFavAdsPOST
	User_id int `json:"User_id"`
	Ads_id  int `json:"Ads_id"`
}

type SigReview struct { // используется в SigReviewPOST
	Ads_id  int    `json:"Ads_id"`
	Rating  int    `json:"Rating"`
	Comment string `json:"Comment"`
	State   int    `json:"State"`
}

type UpdReview struct { // используется в UpdReviewPOST
	Review_id int    `json:"Review_id"`
	Rating    int    `json:"Rating"`
	Comment   string `json:"Comment"`
}

type Order struct { // используется
	Ad_id       int   `json:"Ad_id"`
	Total_price int   `json:"Total_price"`
	Starts_at   int64 `json:"Starts_at"`
	Ends_at     int64 `json:"Ends_at"`
}

type Booking struct { // используется RegOrderHourlyPOST, RegOrderDailyPOST
	Ads_id    int     `json:"Ads_id"`
	Starts_at int64   `json:"Starts_at"`
	Ends_at   int64   `json:"Ends_at"`
	PositionX float64 `json:"PositionX"`
	PositionY float64 `json:"PositionY"`
}

type Bidding struct { // используется BiddingPOST
	Chat_id     int     `json:"Chat_id"`
	Global_rate int     `json:"Global_rate"`
	Start_at    int64   `json:"Start_at"`
	End_at      int64   `json:"End_at"`
	PositionX   float64 `json:"PositionX"`
	PositionY   float64 `json:"PositionY"`
}

type Bidding_id struct { // используется BiddingPOST
	Bidding_id int `json:"Bidding_id"`
}

type Booking_rebookOrderHourly struct { // используется RebookOrderHourly, RebookOrderDailyPOST
	Order_id  int   `json:"Order_id"`
	Starts_at int64 `json:"Starts_at"`
	Ends_at   int64 `json:"Ends_at"`
}

type ComplBooking struct {
	Bookings_id []int `json:"Bookings_id"`
}

type Report struct { // используется в RegReportPOST
	Order_id int `json:"Order_id"`
}

type Passwd struct { // используется в SendCodeForRecoveryPassWithEmailPOST, EnterPasswdPOST
	Passwd_1 string `json:"Passwd_1"`
	Passwd_2 string `json:"Passwd_2"`
}

type WalletHistory struct { // используется в WalletHistoryPOST,
	Typee int `json:"Type"`
}

type MediatorFinishJob struct { // используется в MediatorFinishJobUserPOST и MediatorFinishJobOwnerPOST
	Chat_id int    `json:"Chat_id"`
	Amount  int    `json:"Amount"`
	Comment string `json:"Comment"`
}

type YandexUserInfo struct {
	ID              string   `json:"id"`
	Login           string   `json:"login"`
	DisplayName     string   `json:"display_name"`
	RealName        string   `json:"real_name"`
	FirstName       string   `json:"first_name"`
	LastName        string   `json:"last_name"`
	DefaultEmail    string   `json:"default_email"`
	Emails          []string `json:"emails"`
	Birthday        string   `json:"birthday"`
	DefaultAvatarID string   `json:"default_avatar_id"`
	IsAvatarEmpty   bool     `json:"is_avatar_empty"`
	Psuid           string   `json:"psuid"`
}

type Address struct {
	Name string `json:"name"`
}
