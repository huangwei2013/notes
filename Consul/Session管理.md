参考
	https://www.consul.io/docs/internals/sessions.html	(2019.07.17)

# Session
	Consul 使用 session 机制实现分布式锁。Session 用于支撑节点间互通,健康检查,key/value数据存储。
	同时设计了锁的支持，服从 Chubby的[The Chubby Lock Service for Loosely-Coupled Distributed Systems.](http://research.google.com/archive/chubby.html)
	
## Session 设计

	Consul Session 建立时，同时建立有：
		>> 一个节点的ID
		>> 一个命名节点
		>> 一个健康检查列表
		>> 一个behavior
		>> 一个TTL
		>> 一个延迟锁
	
	Session失效，有如下情况会引发：
		>> 节点注销
		>> 健康检查中任一项被注销
		>> 健康检查中任一项进入 critical 状态
		>> Session 被显示销毁
		>> 设置了 TTL，且超时了
		
	Seesion失效时，对于相关锁的处理，取决于创建时的定义。可选的有
		>> (默认)release
			- 相关锁被释放
			- 锁相关 key 的 ModifyIndex 递增
		>> delete 
			- 锁相关 key 会被直接删除
		
		
## Key/value 完整性

	KV和sessions的完整性，是应用session的首要点。从全局视角看，一个session必须在"创建"之后，才可"被使用"。
	KV操作有一系列API支持：
		>> "获取(Acquire)"操作
			>> 以CAS模式提供(当然，当前lock holder可重复"获取")。
				>> 成功时，才可更新key
				>> 对 LockIndex 递增
				>> 更新持有者为该session
			>> 若session已经为当前持有者(同一次 Acquire 中，多次 CAS)
				>> 更新key
				>> LockIndex 不递增
		>> "释放(Release)"操作
			>> 可被非其持有者释放
				>> 为保留"强制释放"能力
			>> lock被释放，LockIndex 保持不变
			>> lock被释放，ModifyIndex 递增
			>> 清空 lock 的持有者信息(对session的引用)
	需要了解，锁机制只是建议性的。client获取/释放等操作时，也可以不获取锁(取决于系统设计者)。

## 选举
	基于 session 和 KV锁机制，构建起 client 端视角的选举算法。具体参看[选举算法](https://learn.hashicorp.com/consul/developer-configuration/elections)
	
## 预查询(Prepared Query)集成
	预查询可附加到session上，以便清理session时一并处理。