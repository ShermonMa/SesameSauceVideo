/*
 * scripts.go
 * 功能：Redis Lua 脚本常量，供缓存层原子操作复用
 * 时间戳：2026-05-22
 */

package cache

// LuaReleaseLock 原子释放分布式锁脚本
// KEYS[1]: lock_key
// ARGV[1]: 持锁者写入的 UUID
// 仅当锁的值与 UUID 匹配时才删除，防止误释放其他实例的锁
const LuaReleaseLock = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
else
    return 0
end
`
