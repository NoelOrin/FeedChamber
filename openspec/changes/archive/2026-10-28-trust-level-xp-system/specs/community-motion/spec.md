## RENAMED Requirements
- FROM: `### Requirement: 动议采用信誉加权表决`
- TO: `### Requirement: 动议采用等级加权表决`

## MODIFIED Requirements

### Requirement: 动议采用等级加权表决

系统 SHALL 对动议进行公示-表决：每张票按投票人的**信任等级**折算票权（TL0–TL1 → 1 票、TL2–TL3 → 2 票、TL4 → 3 票；该映射 MUST 可配置），同一用户对同一动议仅能投一票；动议通过需满足「加权赞成 > 加权反对」且「加权总参与 ≥ 版面法定人数」。

`users.reputation` 列 MUST 被移除：它全仓没有任何写入点（唯一读取点 `service/motion.go:124`），导致所有用户的票权恒为 1，是一条从未生效的"信誉加权"。票权 MUST 改为由信任等级派生。

#### Scenario: 达法定人数且加权赞成占优

- **WHEN** 某动议加权总参与达到版面法定人数，且加权赞成票数大于加权反对
- **THEN** 动议结算为 `passed`，触发对应结果执行

#### Scenario: 未达法定人数

- **WHEN** 表决期满加权总参与仍低于版面法定人数
- **THEN** 动议结算为 `rejected`，不执行任何结果，不记滥诉

#### Scenario: 单一用户重复投票

- **WHEN** 同一用户对同一动议发起第二次投票
- **THEN** 系统拒绝第二次投票（唯一约束 `motion_votes(motion_id, voter_id)`）

#### Scenario: 高等级用户票权更大

- **WHEN** 等级为 TL3 的用户与等级为 TL1 的用户对同一动议各投一票
- **THEN** 前者 MUST 计 2 票、后者 MUST 计 1 票

#### Scenario: 游客与新手票权相同

- **WHEN** 等级为 TL0 的用户对某动议投票
- **THEN** 其票权 MUST 为 1
