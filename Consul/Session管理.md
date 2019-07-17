# Session
	Consul 使用 session 机制实现分布式锁。Session 用于支撑节点间互通,健康检查,key/value数据存储。
	同时设计了锁的支持，服从 Chubby的[这个文章](The Chubby Lock Service for Loosely-Coupled Distributed Systems.)
	
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
		