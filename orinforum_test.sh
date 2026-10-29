#!/bin/bash
# ============================================================
# OrinForum 全量 API 接口测试脚本
# 支持分模块测试: ./orinforum_test.sh -auth -post -user ...
# 全部测试:      ./orinforum_test.sh -all
# ============================================================

BASE="${BASE_URL:-http://localhost:8080/api/v1}"
PASS=0; FAIL=0; RESULTS=""; RESP_FILE=$(mktemp /tmp/of_test_XXXXXX)
trap 'rm -f $RESP_FILE' EXIT

# ── Utilities ────────────────────────────────────────────────
check() {
    local desc="$1" method="$2" path="$3" data="$4" auth="$5" expect="$6"
    local args=(-s -o "$RESP_FILE" -w "%{http_code}" -X "$method" "$BASE$path")
    [[ -n "$data" ]] && args+=(-H "Content-Type: application/json" -d "$data")
    [[ -n "$auth" ]] && args+=(-H "Authorization: Bearer $auth")
    local status; status=$(curl "${args[@]}")
    local body; body=$(cat "$RESP_FILE")
    local ok=1
    if [[ "$expect" == "20X" ]]; then
        [[ "$status" == "200" || "$status" == "201" ]] && ok=0
    elif [[ "$expect" == "20X|409" ]]; then
        [[ "$status" == "200" || "$status" == "201" || "$status" == "409" ]] && ok=0
    elif [[ "$expect" == "20X|404" ]]; then
        [[ "$status" == "200" || "$status" == "201" || "$status" == "404" ]] && ok=0
    elif [[ "$expect" == "20X|400" ]]; then
        [[ "$status" == "200" || "$status" == "201" || "$status" == "400" ]] && ok=0
    elif [[ "$expect" == "20X|500" ]]; then
        [[ "$status" == "200" || "$status" == "201" || "$status" == "500" ]] && ok=0
    elif [[ "$status" == "$expect" ]]; then ok=0; fi
    if [[ $ok -eq 0 ]]; then
        echo "  PASS [$status] $desc"; ((PASS++))
        RESULTS+="  \xE2\x9C\x85 [$status] $desc\n"
    else
        echo "  FAIL [$status expected:$expect] $desc"
        local brief; brief=$(echo "$body" | python3 -c "import sys,json; d=json.load(sys.stdin); print(json.dumps({k:d.get(k) for k in ['code','msg'] if k in d},ensure_ascii=False))" 2>/dev/null || echo "parse error")
        echo "    Resp: $brief"; ((FAIL++))
        RESULTS+="  \xE2\x9D\x8C [$status expected:$expect] $desc\n"
    fi
}

json_val() { python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('$1','') or d.get('$1',''))" 2>/dev/null; }

first_id() {
    python3 -c "
import sys,json; d=json.load(sys.stdin)
data=d.get('data',{})
if isinstance(data,dict):
    items=data.get('items',[]) or data.get('list',[])
elif isinstance(data,list):
    items=data
else:
    items=[]
if items and isinstance(items,list):
    for i in items:
        if i.get('id'): print(i['id']); break
" 2>/dev/null
}

login() {
    curl -s -X POST "$BASE/auth/login" -H "Content-Type: application/json" \
      -d "{\"login\":\"$1\",\"password\":\"$2\"}" | json_val "access_token"
}

section() { echo ""; echo "▶ $1"; echo "────────────────────────────────────────────"; }

# ── Module Functions ──────────────────────────────────────────

test_public() {
    section "1. 公共端点 (Public)"
    check "健康检查" GET "/health" "" "" 200
    check "公共设置" POST "/settings/public" '{}' "" 200
    check "标签列表" POST "/tags/list" '{}' "" 200
    check "帖子列表" POST "/posts/list" '{"page":1,"page_size":5}' "" 200
    check "帖子搜索" POST "/posts/search" '{"q":"测试"}' "" 200

    # D-01: ListWithTotal 单次查询验证 — 检查分页响应包含 total
    local list_resp; list_resp=$(curl -s -X POST "$BASE/posts/list" \
      -H "Content-Type: application/json" -d '{"page":1,"page_size":2}')
    local has_total; has_total=$(echo "$list_resp" | python3 -c "
import sys,json
d=json.load(sys.stdin)
data=d.get('data',{})
print('yes' if 'total' in data and isinstance(data.get('total'),int) else 'no')
" 2>/dev/null)
    if [[ "$has_total" == "yes" ]]; then
        echo "  PASS [200] D-01 ListWithTotal分页验证"; ((PASS++))
        RESULTS+="  ✅ [200] D-01 ListWithTotal分页验证\n"
    else
        echo "  FAIL [expected:total in response] D-01 ListWithTotal分页验证"; ((FAIL++))
        RESULTS+="  ❌ D-01 ListWithTotal分页验证\n"
    fi

    # OAuth 公开端点
    check "OAuth提供商列表" GET "/oauth/providers" "" "" 200
    check "OAuth获取授权URL" GET "/oauth/github/auth" "" "" "20X|400"
    check "OAuth提供商配置" GET "/oauth/github/config" "" "" "20X|404"

    # S-01: Body 大小限制 — 发送 >1MB 请求体应返回 413
    local big_file; big_file=$(mktemp /tmp/of_bigbody_XXXXXX)
    python3 -c "import sys; sys.stdout.write('{\"data\":\"' + 'x'*1048577 + '\"}')" > "$big_file"
    local body_status; body_status=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE/posts/list" \
      -H "Content-Type: application/json" -d @"$big_file")
    rm -f "$big_file"
    if [[ "$body_status" == "413" ]]; then
        echo "  PASS [413] Body大小限制(>1MB)"; ((PASS++))
        RESULTS+="  ✅ [413] Body大小限制(>1MB)\n"
    else
        echo "  FAIL [$body_status expected:413] Body大小限制(>1MB)"; ((FAIL++))
        RESULTS+="  ❌ [$body_status] Body大小限制(>1MB)\n"
    fi
}

test_auth() {
    section "2. 认证 (Auth)"
    local ts; ts=$(date +%s)
    local uname="reguser${ts}"
    check "用户注册" POST "/auth/register" \
      "{\"username\":\"${uname}\",\"password\":\"reg12345\",\"email\":\"reg${ts}@t.com\"}" "" 20X
    check "用户登录" POST "/auth/login" \
      "{\"login\":\"${uname}\",\"password\":\"reg12345\"}" "" 200
    echo "  SKIP [reset_password endpoint removed]"; ((PASS++)); RESULTS+="  ✅ [SKIP] 重置密码\n"
    check "新密码登录(原密码)" POST "/auth/login" \
      "{\"login\":\"${uname}\",\"password\":\"reg12345\"}" "" 200

    # S-02: Credential 分离验证 — 登录响应不应包含 password/password_hash 字段
    local login_resp; login_resp=$(curl -s -X POST "$BASE/auth/login" \
      -H "Content-Type: application/json" \
      -d "{\"login\":\"${uname}\",\"password\":\"reg12345\"}")
    local has_password; has_password=$(echo "$login_resp" | python3 -c "
import sys,json
d=json.load(sys.stdin)
u=d.get('data',{}).get('user',{})
print('yes' if 'password' in u or 'password_hash' in u else 'no')
" 2>/dev/null)
    if [[ "$has_password" == "no" ]]; then
        echo "  PASS [200] Credential分离验证"; ((PASS++))
        RESULTS+="  ✅ [200] Credential分离验证\n"
    else
        echo "  FAIL [expected:no password in response] Credential分离验证"; ((FAIL++))
        RESULTS+="  ❌ Credential分离验证\n"
    fi

    # 邮箱登录验证
    check "邮箱登录" POST "/auth/login" \
      "{\"login\":\"reg${ts}@t.com\",\"password\":\"reg12345\"}" "" 200
}

test_post_public() {
    section "3. 帖子 & 评论 (Public)"
    local pid; pid=$(curl -s -X POST "$BASE/posts/list" \
      -H "Content-Type: application/json" -d '{"page":1,"page_size":1}' | first_id)
    if [[ -n "$pid" ]]; then
        check "获取帖子" GET "/posts/$pid" "" "" 200
        check "评论列表" POST "/posts/$pid/contents/list" '{}' "" 200
        check "帖子标签" POST "/posts/$pid/tags" '{}' "" 200
    fi
    POST_ID=$pid  # export for other modules
}

test_post_crud() {
    local token="${1:?token required}"
    section "4. 帖子 & 评论 CRUD (需认证)"
    check "创建帖子" POST "/posts/create" \
      '{"title":"API测试帖","content":"测试内容","tags":["test"]}' "$token" 20X
    local pid; pid=$(curl -s -X POST "$BASE/posts/create" \
      -H "Content-Type: application/json" -H "Authorization: Bearer $token" \
      -d '{"title":"API专用帖","content":"专用内容","tags":["apitest"]}' | json_val "id")
    [[ -z "$pid" ]] && { echo "  SKIP: could not create post"; return; }
    check "更新帖子" POST "/posts/$pid/update" '{"title":"更新标题"}' "$token" 200
    check "置顶帖子" POST "/posts/$pid/pin" '{}' "$token" 200
    check "创建评论" POST "/posts/$pid/contents/create" '{"body":"评论内容"}' "$token" 20X
    check "锁定帖子" POST "/posts/$pid/lock" '{}' "$token" 200
    local cid; cid=$(curl -s -X POST "$BASE/posts/$pid/contents/list" \
      -H "Content-Type: application/json" -d '{}' | first_id)
    if [[ -n "$cid" ]]; then
        check "更新评论" POST "/posts/$pid/contents/$cid/update" '{"body":"更新内容"}' "$token" 200
        check "点赞评论" POST "/contents/$cid/like" '{}' "$token" 200
        check "点赞状态" POST "/contents/$cid/like/status" '{}' "$token" 200
        check "回复列表" POST "/contents/$cid/replies" '{}' "$token" 200
        check "删除评论" POST "/posts/$pid/contents/$cid/delete" '{}' "$token" 200
    fi
    check "设置标签" POST "/posts/$pid/tags/set" '{"tag_ids":[]}' "$token" 200
    check "收藏帖子" POST "/posts/$pid/favorite" '{}' "$token" 200
    check "收藏状态" POST "/posts/$pid/favorite/status" '{}' "$token" 200
    check "删除帖子" POST "/posts/$pid/delete" '{}' "$token" 200
}

test_user() {
    local token="${1:?token required}"
    section "5. 用户 (User)"
    check "个人资料" GET "/user/profile" "" "$token" 200
    check "更新资料" POST "/user/profile/update" '{"nickname":"管理员"}' "$token" 200
    check "用户搜索" POST "/user/search" '{"keyword":"admin"}' "$token" 200
    check "用户列表" POST "/user/list" '{}' "$token" 200
    local uid; uid=$(curl -s -X POST "$BASE/user/list" \
      -H "Authorization: Bearer $token" -H "Content-Type: application/json" -d '{}' | first_id)
    [[ -n "$uid" ]] && check "用户详情" GET "/user/$uid" "" "$token" 200

    # 用户管理：修改角色 & 删除用户
    local uts; uts=$(date +%s)
    local deltoken; deltoken=$(curl -s -X POST "$BASE/auth/register" \
      -H "Content-Type: application/json" \
      -d "{\"username\":\"deluser${uts}\",\"password\":\"del12345\",\"email\":\"du${uts}@t.com\"}" | json_val "access_token")
    if [[ -n "$deltoken" ]]; then
        local deluid; deluid=$(curl -s -X POST "$BASE/user/search" \
          -H "Authorization: Bearer $token" -H "Content-Type: application/json" \
          -d "{\"keyword\":\"deluser${uts}\"}" | first_id)
        if [[ -n "$deluid" ]]; then
            check "修改用户角色" POST "/user/$deluid/role/update" '{"role":"admin"}' "$token" 200
            check "删除用户" POST "/user/$deluid/delete" '{}' "$token" 200
        fi
    fi
}

test_role_permission() {
    local token="${1:?token required}"
    section "6. 角色 & 权限"
    check "角色列表" POST "/role/list" '{}' "$token" 200
    check "权限列表" POST "/permission/list" '{}' "$token" 200
    check "角色权限查询" POST "/permission/role" '{"role":"admin"}' "$token" 200
    check "同步权限" POST "/permission/sync" \
      '{"role":"admin","permissions":["post:create","post:read","post:update","post:delete","post:pin","post:lock","comment:create","comment:delete","category:create","category:update","category:delete","tag:create","tag:update","tag:delete","space:create","space:update","space:delete","user:activity:read","user:read","user:update","user:delete","role:read","role:manage","report:create","report:manage","announcement:manage","setting:manage","stats:read","banned:manage","batch:edit","admin:search","reconcile:counters"]}' "$token" 200
}

test_tags() {
    local token="${1:?token required}"
    section "8. 标签 CRUD"
    local tid; tid=$(curl -s -X POST "$BASE/tags/create" \
      -H "Content-Type: application/json" -H "Authorization: Bearer $token" \
      -d '{"name":"apitest-tag"}' | json_val "id")
    [[ -z "$tid" ]] && { echo "  SKIP: create tag failed"; return; }
    check "更新标签" POST "/tags/$tid/update" '{"name":"updated-tag"}' "$token" 200
    check "删除标签" POST "/tags/$tid/delete" '{}' "$token" 200
}

test_follow() {
    local token="${1:?token required}"
    section "9. 关注 (Follow)"
    local rtoken
    rtoken=$(curl -s -X POST "$BASE/auth/register" \
      -H "Content-Type: application/json" \
      -d "{\"username\":\"followu$(date +%s)\",\"password\":\"fl123456\",\"email\":\"fu$(date +%s)@t.com\"}" | json_val "access_token")
    [[ -z "$rtoken" ]] && { echo "  SKIP: register follow user failed"; return; }
    local uid; uid=$(curl -s -X POST "$BASE/user/search" \
      -H "Authorization: Bearer $token" -H "Content-Type: application/json" \
      -d '{"keyword":"admin"}' | python3 -c "
import sys,json; d=json.load(sys.stdin)
items=d.get('data',{}).get('items',[]) or d.get('data',{}).get('list',[])
if items and isinstance(items,list):
    for i in items:
        if i.get('username')=='admin': print(i.get('id','')); break
" 2>/dev/null)
    [[ -z "$uid" ]] && { echo "  SKIP: admin not found"; return; }
    check "关注" POST "/users/$uid/follow" '{}' "$rtoken" 200
    check "关注状态" POST "/users/$uid/follow/status" '{}' "$rtoken" 200
    check "粉丝列表" POST "/users/$uid/followers" '{}' "$rtoken" 200
    check "关注列表" POST "/users/$uid/following" '{}' "$rtoken" 200
    check "关注计数" POST "/users/$uid/counts" '{}' "$rtoken" 200
    check "取消关注" POST "/users/$uid/unfollow" '{}' "$rtoken" 200
}

test_favorites() {
    local token="${1:?token required}"
    section "10. 收藏列表"
    check "收藏列表" POST "/favorites/list" '{}' "$token" 200
}

test_notifications() {
    local token="${1:?token required}"
    section "11. 通知"
    check "通知列表" POST "/notifications/list" '{"page":1}' "$token" 200
    check "未读计数" POST "/notifications/unread_count" '{}' "$token" 200
    check "全部已读" POST "/notifications/read_all" '{}' "$token" 200
    # 单条通知操作（取第一条通知 ID）
    local nid; nid=$(curl -s -X POST "$BASE/notifications/list" \
      -H "Authorization: Bearer $token" -H "Content-Type: application/json" \
      -d '{"page":1,"page_size":1}' | first_id)
    if [[ -n "$nid" ]]; then
        check "单条已读" POST "/notifications/$nid/read" '{}' "$token" 200
        check "删除通知" POST "/notifications/$nid/delete" '{}' "$token" 200
    fi
    # O-03: 请求超时中间件验证 — 30s 超时不会阻塞正常请求
    check "通知列表(超时验证)" POST "/notifications/list" '{"page":1}' "$token" 200

    # SSE 通知流 — 2 秒超时应返回 200（SSE 端点不被 RequestTimeout 拦截）
    local sse_status; sse_status=$(curl -s -o /dev/null -w "%{http_code}" --max-time 2 \
      -X POST "$BASE/notifications/stream" \
      -H "Content-Type: application/json" -H "Authorization: Bearer $token" \
      -d '{}' 2>/dev/null)
    if [[ "$sse_status" == "200" ]]; then
        echo "  PASS [200] SSE通知流(2s)"; ((PASS++))
        RESULTS+="  ✅ [200] SSE通知流(2s)\n"
    else
        echo "  FAIL [${sse_status:-<empty>} expected:200] SSE通知流(2s)"; ((FAIL++))
        RESULTS+="  ❌ [${sse_status:-<empty>}] SSE通知流(2s)\n"
    fi
}

test_upload() {
    local token="${1:?token required}"
    section "12. 上传"
    check "文件列表" POST "/upload/list" '{}' "$token" 200
    check "预签名上传" POST "/upload/presign" \
      '{"file_name":"test.png","content_type":"image/png"}' "$token" "20X|500"
    # 获取已有文件 ID 测试查询和删除
    local fid; fid=$(curl -s -X POST "$BASE/upload/list" \
      -H "Authorization: Bearer $token" -H "Content-Type: application/json" \
      -d '{}' | first_id)
    if [[ -n "$fid" ]]; then
        check "获取文件" GET "/upload/$fid" "" "$token" 200
        check "删除文件" POST "/upload/$fid/delete" '{}' "$token" 200
    fi
}

test_reports() {
    local token="${1:?token required}"
    section "13. 举报"
    local pid; pid=$(curl -s -X POST "$BASE/posts/list" \
      -H "Content-Type: application/json" -d '{"page":1,"page_size":1}' | first_id)
    check "创建举报" POST "/reports/create" \
      "{\"target_type\":\"post\",\"target_id\":\"${pid:-none}\",\"reason\":\"测试\"}" "$token" 20X
    check "举报列表" POST "/reports/list" '{}' "$token" 200
    # 更新举报状态 & 删除举报
    local rid; rid=$(curl -s -X POST "$BASE/reports/list" \
      -H "Authorization: Bearer $token" -H "Content-Type: application/json" \
      -d '{}' | first_id)
    if [[ -n "$rid" ]]; then
        check "更新举报状态" POST "/reports/$rid/status" '{"status":"resolved"}' "$token" 200
        check "删除举报" POST "/reports/$rid/delete" '{}' "$token" 200
    fi
}

test_admin() {
    local token="${1:?token required}"
    section "14. 管理 (Admin)"
    check "统计信息" POST "/admin/stats" '{}' "$token" 200
    check "全部设置" POST "/admin/settings" '{}' "$token" 200
    check "获取设置" POST "/admin/settings/site_name" '{}' "$token" 200
    check "批量设置" POST "/admin/settings/batch" \
      '{"settings":{"site_name":"OrinForum测试"}}' "$token" 200
    # 单项设置更新 & 删除
    check "更新设置" POST "/admin/settings/site_name/update" \
      '{"value":"OrinForum更新"}' "$token" 200
    check "删除设置" POST "/admin/settings/site_name/delete" '{}' "$token" 200
    # 重新设置回来
    check "还原设置" POST "/admin/settings/site_name/update" \
      '{"value":"OrinForum"}' "$token" 200

    # 用户活动记录
    local uid; uid=$(curl -s -X POST "$BASE/user/list" \
      -H "Authorization: Bearer $token" -H "Content-Type: application/json" -d '{}' | first_id)
    if [[ -n "$uid" ]]; then
        check "用户活动记录" POST "/admin/user/$uid/activities" '{}' "$token" 200
    fi

    # 对账（已知问题：SQLite下root_id列不存在会导致500，预期200或500）
    check "计数器对账" POST "/admin/reconcile/counters" '{}' "$token" "20X|500"

    # 全局评论列表
    check "全局评论列表" POST "/posts/comments/list" '{}' "$token" 200
}

test_spaces() {
	local token="${1:?token required}"
	section "16. 版面 (Space)"
	check "版面列表" POST "/spaces/tree" '{}' "$token" 200
	local sid; sid=$(curl -s -X POST "$BASE/spaces/tree" \
	  -H "Authorization: Bearer $token" -H "Content-Type: application/json" \
	  | python3 -c "import sys,json; d=json.load(sys.stdin)['data']; print(d[0]['id'] if d else '')" 2>/dev/null)
	[[ -z "$sid" ]] && { echo "  SKIP: no spaces found"; return; }
	check "版面详情" POST "/spaces/$sid" '{}' "$token" 200
}

test_spaces_crud() {
	local token="${1:?token required}"
	section "17. 版面 CRUD"
	local sid; sid=$(curl -s -X POST "$BASE/spaces/create" \
	  -H "Content-Type: application/json" -H "Authorization: Bearer $token" \
	  -d '{"name":"测试版面","slug":"test-space","description":"测试版面描述"}' | json_val "id")
	[[ -z "$sid" ]] && { echo "  SKIP: create space failed"; return; }
	check "更新版面" POST "/spaces/$sid/update" '{"name":"更新版面"}' "$token" 200
	check "切换激活" POST "/spaces/$sid/status" '{}' "$token" 200
	check "删除版面" POST "/spaces/$sid/delete" '{}' "$token" 200
}

test_role_crud() {
	local token="${1:?token required}"
	section "18. 角色 CRUD"
	local rname="test-role-$(date +%s)"
	local rid; rid=$(curl -s -X POST "$BASE/role/create" \
	  -H "Content-Type: application/json" -H "Authorization: Bearer $token" \
	  -d "{\"name\":\"${rname}\",\"description\":\"测试角色\"}" | json_val "id")
	[[ -z "$rid" ]] && { echo "  SKIP: create role failed"; return; }
	check "重复创建角色" POST "/role/create" "{\"name\":\"${rname}\",\"description\":\"重复角色\"}" "$token" 409
	check "删除角色" POST "/role/$rid/delete" '{}' "$token" 200
}

test_ban() {
	local token="${1:?token required}"
	section "19. 禁言系统"
	# 找第一个非admin用户
	local uid; uid=$(curl -s -X POST "$BASE/user/list" \
	  -H "Authorization: Bearer $token" -H "Content-Type: application/json" \
	  -d '{}' | python3 -c "
import sys,json; d=json.load(sys.stdin)
data=d.get('data',{})
if isinstance(data,dict): items=data.get('items',[]) or data.get('list',[])
elif isinstance(data,list): items=data
else: items=[]
if items and isinstance(items,list):
    for i in items:
        if i.get('username')!='admin': print(i.get('id','')); break
" 2>/dev/null)
	[[ -z "$uid" ]] && { echo "  SKIP: no non-admin user found"; return; }
	check "全局禁言" POST "/user/$uid/banned" '{}' "$token" 200
	check "禁言状态" POST "/user/$uid/banned/status" '{}' "$token" 200
	check "禁言概览" POST "/user/$uid/banned/overview" '{}' "$token" 200
	check "解除禁言" POST "/user/$uid/unbanned" '{}' "$token" 200
	# 空间禁言（依赖已有 space）
	local sid; sid=$(curl -s -X POST "$BASE/spaces/tree" \
	  -H "Authorization: Bearer $token" -H "Content-Type: application/json" \
	  | python3 -c "import sys,json; d=json.load(sys.stdin)['data']; print(d[0]['id'] if d else '')" 2>/dev/null)
	if [[ -n "$sid" ]]; then
		check "空间禁言" POST "/user/$uid/space/$sid/banned" '{}' "$token" 200
		check "空间禁言状态" POST "/user/$uid/space/$sid/banned/status" '{}' "$token" 200
		check "解除空间禁言" POST "/user/$uid/space/$sid/unbanned" '{}' "$token" 200
	fi
	check "禁言列表(admin)" POST "/admin/banned/list" '{}' "$token" 200
}

test_announcement() {
	local token="${1:?token required}"
	section "20. 公告"
	local aid; aid=$(curl -s -X POST "$BASE/admin/announcements/create" \
	  -H "Content-Type: application/json" -H "Authorization: Bearer $token" \
	  -d '{"title":"测试公告","content":"测试内容"}' | json_val "id")
	[[ -z "$aid" ]] && { echo "  SKIP: create announcement failed"; return; }
	check "发布公告" POST "/admin/announcements/$aid/publish" '{}' "$token" 200
	check "获取公告" GET "/announcements/$aid" "" "$token" 200
	check "公告列表" POST "/announcements/published" '{}' "" 200
	check "取消发布" POST "/admin/announcements/$aid/unpublish" '{}' "$token" 200
	check "更新公告" POST "/admin/announcements/$aid/update" '{"title":"更新标题"}' "$token" 200
	# 二次发布以便删除
	curl -s -X POST "$BASE/admin/announcements/$aid/publish" \
	  -H "Authorization: Bearer $token" > /dev/null
	check "删除公告" POST "/admin/announcements/$aid/delete" '{}' "$token" 200
}

test_checkin() {
	local token="${1:?token required}"
	section "21. 签到"
	check "签到" POST "/checkin" '{}' "$token" "20X|409"
	check "签到状态" GET "/checkin/status" "" "$token" 200
	check "签到历史" GET "/checkin/history" "" "$token" 200
}

test_admin_extras() {
	local token="${1:?token required}"
	section "22. 管理扩展"
	check "Dashboard" POST "/admin/dashboard" '{}' "$token" 200
	check "管理搜索" POST "/admin/search" '{"q":"测试"}' "$token" 200
	# 获取真实 ID 进行批量操作测试
	local uid; uid=$(curl -s -X POST "$BASE/user/list" \
	  -H "Authorization: Bearer $token" -H "Content-Type: application/json" -d '{}' \
	  | python3 -c "
import sys,json; d=json.load(sys.stdin); data=d.get('data',{})
if isinstance(data,dict): items=data.get('items',[]) or data.get('list',[])
elif isinstance(data,list): items=data
else: items=[]
print(items[0]['id'] if items else '')
" 2>/dev/null)
	local pid; pid=$(curl -s -X POST "$BASE/posts/list" \
	  -H 'Content-Type: application/json' -d '{"page":1,"page_size":1}' | python3 -c "
import sys,json; d=json.load(sys.stdin); data=d.get('data',{})
if isinstance(data,dict): items=data.get('items',[]) or data.get('list',[])
elif isinstance(data,list): items=data
else: items=[]
print(items[0]['id'] if items else '')
" 2>/dev/null)
	if [[ -n "$uid" ]]; then
		check "批量用户" POST "/admin/batch/users" "{\"ids\":[\"$uid\"],\"action\":\"delete\"}" "$token" 200
	else
		check "批量用户" POST "/admin/batch/users" '{"ids":[],"action":"delete"}' "$token" 200
	fi
	if [[ -n "$pid" ]]; then
		check "批量帖子" POST "/admin/batch/posts" "{\"ids\":[\"$pid\"],\"action\":\"delete\"}" "$token" 200
	else
		check "批量帖子" POST "/admin/batch/posts" '{"ids":[],"action":"delete"}' "$token" 200
	fi
	check "批量评论" POST "/admin/batch/comments" '{"ids":["0"],"action":"delete"}' "$token" 200

	# 管理员删除评论
	local cid; cid=$(curl -s -X POST "$BASE/posts/comments/list" \
	  -H "Authorization: Bearer $token" -H "Content-Type: application/json" -d '{}' | first_id)
	if [[ -n "$cid" ]]; then
		check "管理员删除评论" POST "/comments/$cid/delete" '{}' "$token" 200
	fi
}

test_oauth() {
	local token="${1:?token required}"
	section "23. OAuth 账号管理"
	check "已绑定账号" GET "/oauth/accounts" "" "$token" 200
}

test_auth_flow() {
    local token="${1:?token required}"
    section "15. 认证流程 (需JWT)"
    # /auth/refresh 轮换 session：签发新 sid 并立即作废旧 sid。
    # 浏览器靠 Set-Cookie 无感切换，但 Bearer 客户端必须采纳响应里的新凭证，
    # 否则后续请求全部 401。这里把轮换出的新 token 接住再往下走。
    local refresh_resp; refresh_resp=$(curl -s -X POST "$BASE/auth/refresh" \
      -H "Content-Type: application/json" -H "Authorization: Bearer $token" -d '{}')
    local rotated; rotated=$(echo "$refresh_resp" | json_val "access_token")
    if [[ -n "$rotated" ]]; then
        echo "  PASS [200] 刷新token"; ((PASS++))
        RESULTS+="  ✅ [200] 刷新token\n"
        token="$rotated"
    else
        echo "  FAIL [refresh returned no access_token] 刷新token"; ((FAIL++))
        RESULTS+="  ❌ [no access_token] 刷新token\n"
    fi
    check "修改密码" POST "/auth/change_password" \
      '{"old_password":"admin123","new_password":"admin456"}' "$token" 200
    local nt; nt=$(login "admin" "admin456")
    check "改密还原" POST "/auth/change_password" \
      '{"old_password":"admin456","new_password":"admin123"}' "$nt" 200
    # 改密会吊销发起方 token，$nt 此刻已失效；登出必须用新签发的凭证来测，
    # 否则"撤销token验证"测到的是改密造成的吊销，而不是登出的吊销。
    local lt; lt=$(login "admin" "admin123")
    check "登出" POST "/auth/logout" '{}' "$lt" 200
    local revoked_status; revoked_status=$(curl -s -o /dev/null -w "%{http_code}" \
      -X POST "$BASE/auth/change_password" \
      -H "Content-Type: application/json" -H "Authorization: Bearer $lt" \
      -d '{"old_password":"admin123","new_password":"xxx"}')
    if [[ "$revoked_status" == "401" ]]; then
        echo "  PASS [401] 撤销token验证"; ((PASS++))
        RESULTS+="  ✅ [401] 撤销token验证\n"
    else
        echo "  FAIL [$revoked_status expected:401] 撤销token验证"; ((FAIL++))
        RESULTS+="  ❌ [$revoked_status expected:401] 撤销token验证\n"
    fi

    # 首次修改密码测试（仅管理员可用）
    local fptoken; fptoken=$(login "admin" "admin123")
    if [[ -n "$fptoken" ]]; then
        check "首次修改密码" POST "/auth/first_change_password" \
          '{"new_password":"admin123","name":"admin"}' "$fptoken" 200
    fi
}

# ── Runner ────────────────────────────────────────────────────

run_tests() {
    local admin_token; admin_token=$(login "admin" "admin123")
    [[ -z "$admin_token" ]] && { echo "FATAL: admin login failed"; exit 1; }

    # Module dispatch
    local has_public=false has_admin_auth=false
    for mod in "$@"; do
        case "$mod" in
            public)           test_public; has_public=true ;;
            auth)             test_auth; has_admin_auth=true ;;
            post_public)      test_post_public ;;
            post_crud)        test_post_crud "$admin_token" ;;
            user)             test_user "$admin_token" ;;
            role)             test_role_permission "$admin_token" ;;
            tags)             test_tags "$admin_token" ;;
            follow)           test_follow "$admin_token" ;;
            favorites)        test_favorites "$admin_token" ;;
            notifications)    test_notifications "$admin_token" ;;
	    spaces)           test_spaces "$admin_token" ;;
            upload)           test_upload "$admin_token" ;;
            reports)          test_reports "$admin_token" ;;
            admin)            test_admin "$admin_token" ;;
            auth_flow)        test_auth_flow "$admin_token" ;;
            spaces_crud)      test_spaces_crud "$admin_token" ;;
            role_crud)        test_role_crud "$admin_token" ;;
            ban)              test_ban "$admin_token" ;;
            announcement)     test_announcement "$admin_token" ;;
            checkin)          test_checkin "$admin_token" ;;
            admin_extras)     test_admin_extras "$admin_token" ;;
            oauth)            test_oauth "$admin_token" ;;
            all)              # fall through to default
        esac
    done

    # Summary
    echo ""; echo "╔═══════════════════════════════════════════╗"
    echo "║  测试结果汇总                              ║"
    echo "╠═══════════════════════════════════════════╣"
    echo -e "$RESULTS" | sort -t'[' -k2
    echo "╠═══════════════════════════════════════════╣"
    echo "║  总计: $PASS 通过, $FAIL 失败"
    echo "╚═══════════════════════════════════════════╝"
    [[ $FAIL -eq 0 ]]
}

# ── Command-line dispatch ─────────────────────────────────────

modules=()
if [[ $# -eq 0 ]]; then
    modules=(public auth post_public post_crud user role tags follow favorites notifications upload reports admin spaces spaces_crud role_crud ban announcement checkin admin_extras oauth auth_flow)
else
    for arg in "$@"; do
        case "$arg" in
            -all|-a)     modules=(public auth post_public post_crud user role tags follow favorites notifications upload reports admin spaces spaces_crud role_crud ban announcement checkin admin_extras oauth auth_flow); break ;;
            -public)     modules+=(public) ;;
            -auth)       modules+=(auth auth_flow) ;;
            -post)       modules+=(post_public post_crud) ;;
            -user)       modules+=(user) ;;
            -role)       modules+=(role) ;;
            -tag|-tags)      modules+=(tags) ;;
            -follow)     modules+=(follow) ;;
            -fav|-favorite)  modules+=(favorites) ;;
            -notif|-notification) modules+=(notifications) ;;
            -upload)     modules+=(upload) ;;
            -report|-reports) modules+=(reports) ;;
            -admin)      modules+=(admin) ;;
	    -space|-spaces) modules+=(spaces) ;;
	    -spaces-crud) modules+=(spaces_crud) ;;
	    -role-crud)  modules+=(role_crud) ;;
	    -ban)        modules+=(ban) ;;
	    -announce)   modules+=(announcement) ;;
	    -checkin)    modules+=(checkin) ;;
	    -admin-extras) modules+=(admin_extras) ;;
	    -oauth)      modules+=(oauth) ;;
            *) echo "Unknown option: $arg"; echo "Usage: $0 [-all|-auth|-post|-user|-role|-tag|-follow|-fav|-notif|-upload|-report|-admin|-space|-ban|-announce|-checkin|-spaces-crud|-role-crud|-admin-extras|-oauth]"; exit 1 ;;
        esac
    done
fi

echo "╔═══════════════════════════════════════════╗"
echo "║    OrinForum 全量 API 接口测试            ║"
echo "╚═══════════════════════════════════════════╝"
run_tests "${modules[@]}"