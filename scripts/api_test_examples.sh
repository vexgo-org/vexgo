# VexGo API 测试脚本 (Bash)

# 配置
BASE_URL="http://127.0.0.1:3001/api"
TOKEN = ""

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 辅助函数：打印分隔线
print_separator() {
    echo "=========================================="
}

# 辅助函数：发送 GET 请求
test_get() {
    local url=$1
    shift
    local params=$@
    
    print_separator
    echo -e "${BLUE}GET $url${NC}"
    print_separator
    
    local query_string=""
    if [ $# -gt 0 ]; then
        # 将参数转换为查询字符串
        for param in "$@"; do
            if [ -n "$query_string" ]; then
                query_string="${query_string}&${param}"
            else
                query_string="${param}"
            fi
        done
    fi
    
    local full_url="${BASE_URL}${url}"
    if [ -n "$query_string" ]; then
        full_url="${full_url}?${query_string}"
    fi
    
    echo "URL: ${full_url}"
    
    if [ -n "$TOKEN" ]; then
        curl -s -H "Authorization: Bearer ${TOKEN}" "${full_url}" | jq .
    else
        curl -s "${full_url}" | jq .
    fi
    
    echo ""
}

# 辅助函数：发送 POST 请求
test_post() {
    local url=$1
    local body=$2
    
    print_separator
    echo -e "${BLUE}POST $url${NC}"
    print_separator
    
    local full_url="${BASE_URL}${url}"
    echo "URL: ${full_url}"
    echo "Body: ${body}"
    
    if [ -n "$TOKEN" ]; then
        curl -s -X POST "${full_url}" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${TOKEN}" \
            -d "${body}" | jq .
    else
        curl -s -X POST "${full_url}" \
            -H "Content-Type: application/json" \
            -d "${body}" | jq .
    fi
    
    echo ""
}

# 辅助函数：发送 PUT 请求
test_put() {
    local url=$1
    local body=$2
    
    print_separator
    echo -e "${BLUE}PUT $url${NC}"
    print_separator
    
    local full_url="${BASE_URL}${url}"
    echo "URL: ${full_url}"
    echo "Body: ${body}"
    
    curl -s -X PUT "${full_url}" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${TOKEN}" \
        -d "${body}" | jq .
    
    echo ""
}

# 辅助函数：发送 DELETE 请求
test_delete() {
    local url=$1
    
    print_separator
    echo -e "${BLUE}DELETE $url${NC}"
    print_separator
    
    local full_url="${BASE_URL}${url}"
    echo "URL: ${full_url}"
    
    curl -s -X DELETE "${full_url}" \
        -H "Authorization: Bearer ${TOKEN}" | jq .
    
    echo ""
}

# 辅助函数：上传文件
test_upload() {
    local url=$1
    local file_path=$2
    
    print_separator
    echo -e "${BLUE}POST $url (Upload)${NC}"
    print_separator
    
    local full_url="${BASE_URL}${url}"
    echo "URL: ${full_url}"
    echo "File: ${file_path}"
    
    curl -s -X POST "${full_url}" \
        -H "Authorization: Bearer ${TOKEN}" \
        -F "file=@${file_path}" | jq .
    
    echo ""
}

# ========================================
# 测试用例
# ========================================

# 1. 公共 API 测试
test_public_apis() {
    echo -e "${GREEN}📢 测试公共 API（无需认证）${NC}"
    echo ""
    
    test_get "/posts" "page=1" "limit=5"
    test_get "/posts/1" ""
    test_get "/categories" ""
    test_get "/tags" ""
    test_get "/stats" ""
    test_get "/stats/popular-posts" "limit=5"
    test_get "/stats/latest-posts" "limit=5"
    test_get "/themes" ""
    test_get "/comments/post/1" ""
    test_get "/likes/1" ""
    test_get "/config/general" ""
}

# 2. 验证码测试
test_captcha() {
    echo -e "${GREEN}🔐 测试验证码${NC}"
    echo ""
    
    # 生成验证码
    local captcha_response=$(test_get "/captcha" "")
    local captcha_id=$(echo "${captcha_response}" | jq -r '.captcha.id')
    local captcha_token=$(echo "${captcha_response}" | jq -r '.captcha.token')
    local captcha_x=$(echo "${captcha_response}" | jq -r '.captcha.x')
    
    echo "Captcha ID: ${captcha_id}"
    echo "Captcha Token: ${captcha_token}"
    echo "Captcha X: ${captcha_x}"
    echo ""
    
    # 验证验证码
    test_post "/captcha/verify" "{\"captcha_id\":\"${captcha_id}\",\"captcha_token\":\"${captcha_token}\",\"captcha_x\":${captcha_x}}"
}

# 3. 认证 API 测试
test_auth_apis() {
    echo -e "${GREEN}🔑 测试认证 API${NC}"
    echo ""
    
    # 注册
    test_post "/auth/register" '{"email":"test@example.com","password":"password123","username":"testuser","captcha_id":"","captcha_token":"","captcha_x":0}'
    
    # 登录（获取 token）
    echo -e "${YELLOW}请执行登录并复制 token:${NC}"
    test_post "/auth/login" '{"email":"test@example.com","password":"password123","captcha_id":"","captcha_token":"","captcha_x":0}'
    
    echo ""
    echo -e "${YELLOW}⚠️  请手动复制上面的 token 并更新脚本中的 TOKEN 变量${NC}"
    echo ""
}

# 4. 需要认证的 API 测试（需要先设置 token）
test_authenticated_apis() {
    if [ -z "$TOKEN" ]; then
        echo -e "${RED}❌ 请先设置 TOKEN 变量！${NC}"
        return
    fi
    
    echo -e "${GREEN}🔒 测试需要认证的 API${NC}"
    echo ""
    
    # 用户信息
    test_get "/auth/me" ""
    test_get "/auth/user" ""
    
    # 帖子管理
    test_get "/posts/user/my-posts" "page=1" "limit=10"
    test_get "/posts/drafts" "page=1" "limit=10"
    
    # 创建帖子
    test_post "/posts" '{"title":"Test Post","content":"This is a test post content","category_id":1,"tag_ids":[1],"status":"draft"}'
    
    # 评论
    test_post "/comments" '{"postId":1,"content":"Test comment","parentId":null}'
    
    # 点赞
    test_post "/likes/1" "{}"
    
    # 用户设置
    test_put "/auth/profile" '{"username":"newname","bio":"My bio"}'
    test_put "/auth/password" '{"current_password":"oldpass","new_password":"newpass"}'
    
    # 获取我的文件
    test_get "/upload/my-files" "page=1" "limit=10"
}

# 5. 管理员 API 测试（需要 admin/super_admin 权限）
test_admin_apis() {
    if [ -z "$TOKEN" ]; then
        echo -e "${RED}❌ 请先设置 TOKEN 变量！${NC}"
        return
    fi
    
    echo -e "${GREEN}👑 测试管理员 API${NC}"
    echo ""
    
    # 用户管理
    test_get "/users" "page=1" "limit=10"
    test_put "/users/2/role" '{"role":"admin"}'
    # test_delete "/users/2" ""
    
    # 分类和标签管理
    test_post "/categories" '{"name":"New Category","slug":"new-category","description":"Description"}'
    test_post "/tags" '{"name":"newtag","slug":"newtag"}'
    
    # 配置管理
    test_get "/config/smtp" ""
    test_get "/config/ai" ""
    test_get "/config/theme" ""
    test_get "/config/general" ""
    
    # 审核管理
    test_get "/moderation/pending" "page=1" "limit=10"
    test_get "/moderation/comments/pending" "page=1" "limit=10"
    test_put "/moderation/approve/1" "{}"
    test_put "/moderation/reject/1" '{"rejection_reason":"Reason"}'
    test_put "/moderation/resubmit/1" "{}"
    
    test_put "/moderation/comments/approve/1" "{}"
    test_put "/moderation/comments/reject/1" '{"rejection_reason":"Reason"}'
    
    # 审核配置
    test_get "/moderation/comments/config" ""
    test_put "/moderation/comments/config" '{"enabled":true,"model_provider":"openai","api_key":"your_key","api_endpoint":"https://api.openai.com/v1/chat/completions","model_name":"gpt-3.5-turbo","moderation_prompt":"Please review...","block_keywords":"spam,ad","auto_approve_enabled":true,"min_score_threshold":0.5}'
}

# ========================================
# 主程序
# ========================================

# 检查 jq 是否安装
if ! command -v jq &> /dev/null; then
    echo -e "${RED}错误: jq 未安装，请先安装 jq${NC}"
    echo "Ubuntu/Debian: sudo apt-get install jq"
    echo "macOS: brew install jq"
    exit 1
fi

# 显示菜单
show_menu() {
    echo ""
    echo "VexGo API 测试脚本"
    echo "================================"
    echo "1. 测试公共 API"
    echo "2. 测试验证码"
    echo "3. 测试认证 API（注册/登录）"
    echo "4. 测试需要认证的 API"
    echo "5. 测试管理员 API"
    echo "6. 运行所有测试"
    echo "0. 退出"
    echo "================================"
    read -p "请选择 (0-6): " choice
}

# 运行选择的测试
run_test() {
    case $1 in
        1) test_public_apis ;;
        2) test_captcha ;;
        3) test_auth_apis ;;
        4) test_authenticated_apis ;;
        5) test_admin_apis ;;
        6) 
            test_public_apis
            test_captcha
            test_auth_apis
            if [ -n "$TOKEN" ]; then
                test_authenticated_apis
                test_admin_apis
            else
                echo -e "${YELLOW}⚠️  跳过需要认证的测试，请先设置 TOKEN 变量${NC}"
            fi
            ;;
        0) exit 0 ;;
        *) echo "无效选择" ;;
    esac
}

# 如果提供了参数，直接运行对应的测试
if [ $# -gt 0 ]; then
    run_test $1
    exit 0
fi

# 交互模式
while true; do
    show_menu
    run_test $choice
done
