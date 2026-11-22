package types

import "time"

// CookieState 用于把校园登录过程中拿到的 cookie 序列化到缓存里。
type CookieState struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	Secure   bool   `json:"secure"`
	HttpOnly bool   `json:"http_only"`
}

// PreAuthSession 表示 PreAuthorize 阶段缓存的会话数据。
type PreAuthSession struct {
	AuthSessionID string        `json:"auth_session_id"`
	CaptchaUID    string        `json:"captcha_uid"`
	CaptchaBase64 string        `json:"captcha_base64"`
	Cookies       []CookieState `json:"cookies"`
	CreatedAt     time.Time     `json:"created_at"`
}

// AuthorizedTicket 表示校园授权成功后生成的临时票据。
type AuthorizedTicket struct {
	SchoolAuthTicket string    `json:"school_auth_ticket"`
	StudentNo        string    `json:"student_no"`
	StudentName      string    `json:"student_name"`
	StudentStatus    int32     `json:"student_status"`
	AdmissionAt      time.Time `json:"admission_at"`
	GraduationAt     time.Time `json:"graduation_at"`
	TGT              string    `json:"tgt"`
	Ticket           string    `json:"ticket"`
	CreatedAt        time.Time `json:"created_at"`
}

// SchoolAccount 表示 school_accounts 表记录。
type SchoolAccount struct {
	ID            int64     `json:"id"`
	UserID        int64     `json:"user_id"`
	StudentNo     string    `json:"student_no"`
	StudentName   string    `json:"student_name"`
	Password      string    `json:"password"`
	StudentStatus int32     `json:"student_status"`
	BindStatus    int32     `json:"bind_status"`
	AdmissionAt   time.Time `json:"admission_at"`
	GraduationAt  time.Time `json:"graduation_at"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// SchoolSession 表示学校侧缓存会话。
type SchoolSession struct {
	UserID     int64     `json:"user_id"`
	StudentNo  string    `json:"student_no"`
	TGT        string    `json:"tgt"`
	Ticket     string    `json:"ticket"`
	Cookies    string    `json:"cookies"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	LastSynced time.Time `json:"last_synced"`
}

// StudentProfilePayload 表示从学校页面解析后的个人信息。
type StudentProfilePayload struct {
	FetchedAt     string            `json:"fetched_at"`
	StudentID     string            `json:"student_id"`
	StudentIDCode string            `json:"student_id_code"`
	HiddenFields  map[string]string `json:"hidden_fields"`
	Fields        map[string]string `json:"fields"`
}

// StudentScheduleItem 表示学校课表中的单条课程记录。
type StudentScheduleItem struct {
	CourseName   string `json:"course_name"`
	Teacher      string `json:"teacher"`
	Classroom    string `json:"classroom"`
	Weekday      string `json:"weekday"`
	StartSection string `json:"start_section"`
	EndSection   string `json:"end_section"`
	Weeks        string `json:"weeks"`
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
}

// ScheduleSectionTime 表示学校课表中的节次时间定义。
type ScheduleSectionTime struct {
	Section   string `json:"section"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}
