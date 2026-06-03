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
items=d.get('data',d.get('items',[]) or d.get('list',[]))
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
    check "分类列表" POST "/categories/list" '{}' "" 200
    check "标签列表" POST "/tags/list" '{}' "" 200
    check "帖子列表" POST "/posts/list" '{"page":1,"page_size":5}' "" 200
    check "帖子搜索" POST "/posts/search" '{"q":"测试"}' "" 200
}

test_auth() {
    section "2. 认证 (Auth)"
    local ts; ts=$(date +%s)
    local uname="reguser${ts}"
    check "用户注册" POST "/auth/register" \
      "{\"username\":\"${uname}\",\"password\":\"reg123\",\"email\":\"reg${ts}@t.com\"}" "" 20X
    check "用户登录" POST "/auth/login" \
      "{\"login\":\"${uname}\",\"password\":\"reg123\"}" "" 200
    check "重置密码" POST "/auth/reset_password" \
      "{\"username\":\"${uname}\",\"new_password\":\"reg456\"}" "" 200
    check "新密码登录" POST "/auth/login" \
      "{\"login\":\"${uname}\",\"password\":\"reg456\"}" "" 200
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
      '{"title":"API测试帖","category_id":"","tags":["test"]}' "$token" 20X
    local pid; pid=$(curl -s -X POST "$BASE/posts/create" \
      -H "Content-Type: application/json" -H "Authorization: Bearer $token" \
      -d '{"title":"API专用帖","category_id":"","tags":["apitest"]}' | json_val "id")
    [[ -z "$pid" ]] && { echo "  SKIP: could not create post"; return; }
    check "更新帖子" POST "/posts/$pid/update" '{"title":"更新标题"}' "$token" 200
    check "置顶帖子" POST "/posts/$pid/pin" '{}' "$token" 200
    check "锁定帖子" POST "/posts/$pid/lock" '{}' "$token" 200
    check "创建评论" POST "/posts/$pid/contents/create" '{"content":"评论内容"}' "$token" 20X
    local cid; cid=$(curl -s -X POST "$BASE/posts/$pid/contents/list" \
      -H "Content-Type: application/json" -d '{}' | first_id)
    if [[ -n "$cid" ]]; then
        check "更新评论" POST "/posts/$pid/contents/$cid/update" '{"content":"更新内容"}' "$token" 200
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
}

test_role_permission() {
    local token="${1:?token required}"
    section "6. 角色 & 权限"
    check "角色列表" POST "/role/list" '{}' "$token" 200
    check "权限列表" POST "/permission/list" '{}' "$token" 200
    check "角色权限查询" POST "/permission/role" '{"role":"admin"}' "$token" 200
    check "同步权限" POST "/permission/sync" \
      '{"role":"admin","permissions":["post:create","post:read","post:update","post:delete","post:pin","post:lock","comment:create","comment:delete","category:create","category:delete","user:read","user:update","user:delete","role:read","role:manage","report:create","report:manage"]}' "$token" 200
}

test_categories() {
    local token="${1:?token required}"
    section "7. 分类 CRUD"
    local cid; cid=$(curl -s -X POST "$BASE/categories/create" \
      -H "Content-Type: application/json" -H "Authorization: Bearer $token" \
      -d '{"name":"测试分类","description":"描述"}' | json_val "id")
    [[ -z "$cid" ]] && { echo "  SKIP: create category failed"; return; }
    check "更新分类" POST "/categories/$cid/update" '{"name":"更新分类"}' "$token" 200
    check "删除分类" POST "/categories/$cid/delete" '{}' "$token" 200
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
      -d '{"username":"followu","password":"fl123","email":"fu@t.com"}' | json_val "access_token")
    [[ -z "$rtoken" ]] && { echo "  SKIP: register follow user failed"; return; }
    local uid; uid=$(curl -s -X POST "$BASE/user/search" \
      -H "Authorization: Bearer $token" -H "Content-Type: application/json" \
      -d '{"keyword":"admin"}' | python3 -c "
import sys,json; d=json.load(sys.stdin)
items=d.get('data',{}).get('list',[])
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
}

test_upload() {
    local token="${1:?token required}"
    section "12. 上传"
    check "文件列表" POST "/upload/list" '{}' "$token" 200
}

test_reports() {
    local token="${1:?token required}"
    section "13. 举报"
    local pid; pid=$(curl -s -X POST "$BASE/posts/list" \
      -H "Content-Type: application/json" -d '{"page":1,"page_size":1}' | first_id)
    check "创建举报" POST "/reports/create" \
      "{\"targetType\":\"post\",\"targetId\":\"${pid:-none}\",\"reason\":\"测试\"}" "$token" 20X
    check "举报列表" POST "/reports/list" '{}' "$token" 200
}

test_admin() {
    local token="${1:?token required}"
    section "14. 管理 (Admin)"
    check "统计信息" POST "/admin/stats" '{}' "$token" 200
    check "全部设置" POST "/admin/settings" '{}' "$token" 200
    check "获取设置" POST "/admin/settings/site_name" '{}' "$token" 200
    check "批量设置" POST "/admin/settings/batch" \
      '{"settings":{"site_name":"OrinForum测试"}}' "$token" 200
}

test_auth_flow() {
    local token="${1:?token required}"
    section "15. 认证流程 (需JWT)"
    check "刷新token" POST "/auth/refresh" '{}' "$token" 200
    check "登出" POST "/auth/logout" '{}' "$token" 200
    check "修改密码" POST "/auth/change_password" \
      '{"old_password":"admin123","new_password":"admin456"}' "$token" 200
    local nt; nt=$(login "admin" "admin456")
    check "改密还原" POST "/auth/change_password" \
      '{"old_password":"admin456","new_password":"admin123"}' "$nt" 200
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
            categories)       test_categories "$admin_token" ;;
            tags)             test_tags "$admin_token" ;;
            follow)           test_follow "$admin_token" ;;
            favorites)        test_favorites "$admin_token" ;;
            notifications)    test_notifications "$admin_token" ;;
            upload)           test_upload "$admin_token" ;;
            reports)          test_reports "$admin_token" ;;
            admin)            test_admin "$admin_token" ;;
            auth_flow)        test_auth_flow "$admin_token" ;;
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
    modules=(public auth post_public post_crud user role categories tags follow favorites notifications upload reports admin auth_flow)
else
    for arg in "$@"; do
        case "$arg" in
            -all|-a)     modules=(public auth post_public post_crud user role categories tags follow favorites notifications upload reports admin auth_flow); break ;;
            -public)     modules+=(public) ;;
            -auth)       modules+=(auth auth_flow) ;;
            -post)       modules+=(post_public post_crud) ;;
            -user)       modules+=(user) ;;
            -role)       modules+=(role) ;;
            -cat|-category)  modules+=(categories) ;;
            -tag|-tags)      modules+=(tags) ;;
            -follow)     modules+=(follow) ;;
            -fav|-favorite)  modules+=(favorites) ;;
            -notif|-notification) modules+=(notifications) ;;
            -upload)     modules+=(upload) ;;
            -report|-reports) modules+=(reports) ;;
            -admin)      modules+=(admin) ;;
            *) echo "Unknown option: $arg"; echo "Usage: $0 [-all|-auth|-post|-user|-role|-cat|-tags|-follow|-fav|-notif|-upload|-report|-admin]"; exit 1 ;;
        esac
    done
fi

echo "╔═══════════════════════════════════════════╗"
echo "║    OrinForum 全量 API 接口测试            ║"
echo "╚═══════════════════════════════════════════╝"
run_tests "${modules[@]}"