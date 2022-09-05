# Lab 4A The Shard Master

注：Lab description 参照的是 2020 版本的课程，但是代码使用的是 2022 版本。在 2022 版本中 shardmaster 更名为 shardctrler。

4A 的任务要求是完成 shardmaster 的设计。shardmaster 的功能是进行配置的管理，需要实现四个 RPC 接口 ：Join，Leave，Move，Query。

TA 给出了如下的 Hint：

- 可以从一个简约版本的 kvraft server开始
- 应该实现对于客户端请求的重复检测。这一点主要是为了应对不可靠网络的测试。
- Go 中的 map 是引用。如果把一个 map 变量赋值给另一个，他们二者都指向同一个 map。所以在依据以前的配置创建新配置的时候，需要构建一个新的 map 对象进行赋值拷贝（也就是深拷贝）。

---

Lab3 中利用 raft 协议实现了一个 KV 数据存储服务，是基于单一 raft 集群的。但是随着数据的增长，raft leader 的负载加重，服务可能会出现不可用的状态。这时就需要考虑将服务负载进行分片。Lab 4的内容就是将数据按照某种映射方式分开存储到不同的 raft group 上。

Lab 4A 实现 shardmaster 服务，其作用是提供集群的配置管理服务，并且尽可能少的移动分片，记录了每个 raft group 的集群信息和每个 shard 属于哪个 group。具体实现是通过 raft 来维护一个 configs 的集合。

config 的内容如下：

- config num 配置的编号
- shards 分片的集群信息，如 shards[3] = 2 的意思就是分片3的数据存储在集群2上
- groups 集群成员信息，如 Group[3]=['server1','server2']，说明 gid = 3 的集群 Group3 包含两台名称为 server1 和 server2 的机器

本质上来说，shardmaster 的实现和 kvraft 的实现是一致的。在master这一层并不涉及具体的存储内容更改，只是在配置上的操作，可以看作是某种程度上的 raftkv 实现。并且在这一层，由于config大小比较小，并且在启动时可以自己配置，可以不用实现 snapshot。

然后主要就是对四个 RPC 请求的实现了。在这里的实现中依然是把config的管理抽象成了状态机。对于 Query 和 Move，直接实现即可。

对于 Join 和 Leave，Lab 中要求了需要在变更配置的时候让shard的分布更加均匀，并且尽量产生较少的迁移任务。对于Join来说，可以通过每次选取一个拥有最多shard的raft组，分配其中一个shard给拥有最少shard的raft组来进行平均。对于 Leave，如果离开后集群中没有 raft 组了，则将分片所属的 raft 组都变更为无效的状态。