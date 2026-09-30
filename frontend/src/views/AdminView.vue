<template>
  <div class="page">
    <el-card shadow="never">
      <template #header>
        <div class="hdr">
          <span>运营配置</span>
          <el-button @click="reload">刷新</el-button>
        </div>
      </template>
      <el-tabs v-model="tab">
        <el-tab-pane label="席位管理" name="seats">
          <el-form :inline="true">
            <el-form-item label="车次">
              <el-select v-model="seatTrainId" style="width:140px" @change="loadSeats">
                <el-option v-for="t in trains" :key="t.id" :label="t.train_no" :value="t.id" />
              </el-select>
            </el-form-item>
            <el-form-item>
              <el-button @click="loadSeats">刷新席位</el-button>
            </el-form-item>
            <el-form-item label="重建投影日期">
              <el-date-picker v-model="projDate" type="date" value-format="YYYY-MM-DD" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="rebuildProj">重建余票投影</el-button>
            </el-form-item>
          </el-form>
          <el-table :data="seats" size="small" height="420">
            <el-table-column prop="id" label="ID" width="70" />
            <el-table-column prop="carriage_no" label="车厢" width="80" />
            <el-table-column prop="seat_no" label="座位" width="100" />
            <el-table-column prop="seat_type" label="席别" width="90" />
            <el-table-column prop="position" label="位置" width="80" />
            <el-table-column label="状态" width="100">
              <template #default="{ row }">
                <el-tag :type="row.blocked ? 'danger' : 'success'">{{ row.blocked ? '已封锁' : '可售' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="block_reason" label="原因" />
            <el-table-column label="操作" width="120">
              <template #default="{ row }">
                <el-button v-if="!row.blocked" link type="danger" @click="blockSeat(row)">封锁</el-button>
                <el-button v-else link type="primary" @click="unblockSeat(row)">解封</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>

        <el-tab-pane label="按日封锁" name="dayblock">
          <el-form :inline="true" :model="dayForm" @submit.prevent="saveDayBlock">
            <el-form-item label="车次">
              <el-select v-model="dayForm.train_id" style="width:120px" @change="onDayTrainChange">
                <el-option v-for="t in trains" :key="t.id" :label="t.train_no" :value="t.id" />
              </el-select>
            </el-form-item>
            <el-form-item label="日期">
              <el-date-picker v-model="dayForm.travel_date" type="date" value-format="YYYY-MM-DD" />
            </el-form-item>
            <el-form-item label="座位">
              <el-select v-model="dayForm.seat_id" filterable style="width:160px">
                <el-option v-for="s in seats" :key="s.id" :label="`${s.seat_no} ${s.seat_type}`" :value="s.id" />
              </el-select>
            </el-form-item>
            <el-form-item label="原因"><el-input v-model="dayForm.reason" style="width:120px" /></el-form-item>
            <el-form-item><el-button type="primary" native-type="submit">按日封锁</el-button></el-form-item>
            <el-form-item><el-button @click="loadDayBlocks">刷新</el-button></el-form-item>
          </el-form>
          <el-table :data="dayBlocks" size="small">
            <el-table-column prop="id" label="ID" width="70" />
            <el-table-column prop="train_id" label="车次ID" width="80" />
            <el-table-column prop="seat_id" label="座位ID" width="90" />
            <el-table-column prop="travel_date" label="日期" width="120" />
            <el-table-column prop="reason" label="原因" />
            <el-table-column label="操作" width="90">
              <template #default="{ row }">
                <el-button link type="danger" @click="delDayBlock(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>

        <el-tab-pane label="区段配额" name="quota">
          <el-form :inline="true" :model="quotaForm" @submit.prevent="saveQuota">
            <el-form-item label="车次">
              <el-select v-model="quotaForm.train_id" style="width:140px">
                <el-option v-for="t in trains" :key="t.id" :label="t.train_no" :value="t.id" />
              </el-select>
            </el-form-item>
            <el-form-item label="席别"><el-input v-model="quotaForm.seat_type" placeholder="二等座" style="width:100px" /></el-form-item>
            <el-form-item label="区段"><el-input-number v-model="quotaForm.seg_index" :min="0" /></el-form-item>
            <el-form-item label="上限"><el-input-number v-model="quotaForm.max_sold" :min="0" /></el-form-item>
            <el-form-item><el-button type="primary" native-type="submit">保存</el-button></el-form-item>
          </el-form>
          <el-table :data="quotas" size="small">
            <el-table-column prop="id" label="ID" width="70" />
            <el-table-column prop="train_id" label="车次ID" width="90" />
            <el-table-column prop="seat_type" label="席别" />
            <el-table-column prop="seg_index" label="区段" width="80" />
            <el-table-column prop="max_sold" label="上限" width="80" />
            <el-table-column label="操作" width="90">
              <template #default="{ row }">
                <el-button link type="danger" @click="delQuota(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>

        <el-tab-pane label="放票策略" name="sale">
          <el-form label-width="100px" :model="saleForm" style="max-width:480px">
            <el-form-item label="名称"><el-input v-model="saleForm.name" /></el-form-item>
            <el-form-item label="提前天数"><el-input-number v-model="saleForm.advance_days" :min="0" /></el-form-item>
            <el-form-item label="开售时">
              <el-input-number v-model="saleForm.open_hour" :min="0" :max="23" /> :
              <el-input-number v-model="saleForm.open_minute" :min="0" :max="59" />
            </el-form-item>
            <el-form-item label="启用"><el-switch v-model="saleForm.enabled" /></el-form-item>
            <el-form-item><el-button type="primary" @click="saveSale">保存策略</el-button></el-form-item>
          </el-form>
          <el-table :data="sales" size="small">
            <el-table-column prop="id" label="ID" width="70" />
            <el-table-column prop="name" label="名称" />
            <el-table-column prop="advance_days" label="提前天" width="90" />
            <el-table-column label="开售" width="100">
              <template #default="{ row }">{{ row.open_hour }}:{{ String(row.open_minute).padStart(2, '0') }}</template>
            </el-table-column>
            <el-table-column prop="enabled" label="启用" width="80" />
          </el-table>
        </el-tab-pane>

        <el-tab-pane label="放票波次" name="waves">
          <el-form :inline="true" :model="waveForm" @submit.prevent="saveWave">
            <el-form-item label="名称"><el-input v-model="waveForm.name" style="width:140px" /></el-form-item>
            <el-form-item label="提前天"><el-input-number v-model="waveForm.advance_days" :min="0" /></el-form-item>
            <el-form-item label="时">
              <el-input-number v-model="waveForm.open_hour" :min="0" :max="23" />
            </el-form-item>
            <el-form-item label="分"><el-input-number v-model="waveForm.open_minute" :min="0" :max="59" /></el-form-item>
            <el-form-item label="席别"><el-input v-model="waveForm.seat_type" placeholder="空=全部" style="width:90px" /></el-form-item>
            <el-form-item label="车次ID"><el-input-number v-model="waveForm.train_id" :min="0" /></el-form-item>
            <el-form-item label="区段">
              <el-input-number v-model="waveForm.seg_index" :min="-1" />
              <span class="hint">-1=全部</span>
            </el-form-item>
            <el-form-item label="放票%"><el-input-number v-model="waveForm.release_pct" :min="1" :max="100" /></el-form-item>
            <el-form-item label="启用"><el-switch v-model="waveForm.enabled" /></el-form-item>
            <el-form-item><el-button type="primary" native-type="submit">保存波次</el-button></el-form-item>
          </el-form>
          <el-table :data="waves" size="small">
            <el-table-column prop="id" label="ID" width="60" />
            <el-table-column prop="name" label="名称" min-width="120" />
            <el-table-column prop="advance_days" label="提前" width="70" />
            <el-table-column label="开售" width="90">
              <template #default="{ row }">{{ row.open_hour }}:{{ String(row.open_minute).padStart(2,'0') }}</template>
            </el-table-column>
            <el-table-column prop="seat_type" label="席别" width="90" />
            <el-table-column prop="train_id" label="车次" width="70" />
            <el-table-column label="区段" width="70">
              <template #default="{ row }">{{ row.seg_index == null ? '全' : row.seg_index }}</template>
            </el-table-column>
            <el-table-column prop="release_pct" label="%" width="60" />
            <el-table-column prop="enabled" label="启用" width="70" />
            <el-table-column label="操作" width="90">
              <template #default="{ row }">
                <el-button link type="danger" @click="delWave(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>

        <el-tab-pane label="风控黑名单" name="risk">
          <el-form :inline="true" :model="riskForm" @submit.prevent="saveRisk">
            <el-form-item label="类型">
              <el-select v-model="riskForm.kind" style="width:100px">
                <el-option label="user" value="user" />
                <el-option label="ip" value="ip" />
              </el-select>
            </el-form-item>
            <el-form-item label="值"><el-input v-model="riskForm.value" placeholder="用户ID或IP" /></el-form-item>
            <el-form-item label="原因"><el-input v-model="riskForm.reason" /></el-form-item>
            <el-form-item><el-button type="primary" native-type="submit">添加</el-button></el-form-item>
          </el-form>
          <el-table :data="risks" size="small">
            <el-table-column prop="id" label="ID" width="70" />
            <el-table-column prop="kind" label="类型" width="80" />
            <el-table-column prop="value" label="值" />
            <el-table-column prop="reason" label="原因" />
            <el-table-column label="操作" width="90">
              <template #default="{ row }">
                <el-button link type="danger" @click="delRisk(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>

        <el-tab-pane label="网关状态" name="gw">
          <el-descriptions :column="2" border>
            <el-descriptions-item label="当前下单速率">{{ gw.book_rate_per_sec }} /s</el-descriptions-item>
            <el-descriptions-item label="后端延迟">{{ gw.backend_latency_ms }} ms</el-descriptions-item>
            <el-descriptions-item label="速率下限">{{ gw.min_rate }}</el-descriptions-item>
            <el-descriptions-item label="速率上限">{{ gw.max_rate }}</el-descriptions-item>
          </el-descriptions>
          <el-button style="margin-top:12px" @click="loadGw">刷新网关</el-button>
        </el-tab-pane>
      </el-tabs>
    </el-card>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import * as api from '../api'

const tab = ref('seats')
const trains = ref([])
const seats = ref([])
const seatTrainId = ref(null)
const projDate = ref('')
const quotas = ref([])
const sales = ref([])
const waves = ref([])
const dayBlocks = ref([])
const risks = ref([])
const gw = ref({})
const quotaForm = reactive({ train_id: null, seat_type: '二等座', seg_index: 0, max_sold: 20 })
const saleForm = reactive({ id: 0, name: '默认放票', advance_days: 15, open_hour: 8, open_minute: 0, enabled: true })
const riskForm = reactive({ kind: 'ip', value: '', reason: '' })
const dayForm = reactive({ train_id: null, seat_id: null, travel_date: '', reason: '检修' })
const waveForm = reactive({
  name: '新波次', advance_days: 15, open_hour: 8, open_minute: 0,
  seat_type: '二等座', train_id: 0, seg_index: -1, release_pct: 50, enabled: true, sort_order: 10,
})

async function reload() {
  trains.value = await api.adminListTrains()
  quotas.value = await api.adminListQuotas()
  sales.value = await api.adminListSalePolicies()
  waves.value = await api.adminListSaleWaves()
  risks.value = await api.adminListRiskBlocks()
  if (trains.value.length && !quotaForm.train_id) quotaForm.train_id = trains.value[0].id
  if (trains.value.length && !seatTrainId.value) seatTrainId.value = trains.value[0].id
  if (trains.value.length && !dayForm.train_id) dayForm.train_id = trains.value[0].id
  if (sales.value.length) Object.assign(saleForm, sales.value[0])
  if (!projDate.value) {
    const d = new Date()
    d.setDate(d.getDate() + 1)
    projDate.value = d.toISOString().slice(0, 10)
  }
  if (!dayForm.travel_date) dayForm.travel_date = projDate.value
  await loadSeats()
  await loadDayBlocks()
  await loadGw()
}

async function loadSeats() {
  if (!seatTrainId.value) return
  seats.value = await api.adminListSeats(seatTrainId.value)
}

async function onDayTrainChange() {
  seatTrainId.value = dayForm.train_id
  await loadSeats()
  await loadDayBlocks()
}

async function loadDayBlocks() {
  dayBlocks.value = await api.adminListDayBlocks({
    train_id: dayForm.train_id || undefined,
    travel_date: dayForm.travel_date || undefined,
  })
}

async function saveDayBlock() {
  await api.adminUpsertDayBlock({ ...dayForm })
  ElMessage.success('已按日封锁')
  await loadDayBlocks()
}

async function delDayBlock(row) {
  await api.adminDeleteDayBlock(row.id)
  await loadDayBlocks()
}

async function saveWave() {
  const payload = { ...waveForm }
  if (payload.seg_index < 0) payload.seg_index = null
  await api.adminUpsertSaleWave(payload)
  ElMessage.success('波次已保存')
  waves.value = await api.adminListSaleWaves()
}

async function delWave(row) {
  await api.adminDeleteSaleWave(row.id)
  waves.value = await api.adminListSaleWaves()
}

async function blockSeat(row) {
  const { value } = await ElMessageBox.prompt('封锁原因', '封锁座位', { inputValue: '设备故障' })
  await api.adminBlockSeat(row.id, { reason: value || '封锁' })
  ElMessage.success('已封锁')
  await loadSeats()
}

async function unblockSeat(row) {
  await api.adminUnblockSeat(row.id)
  ElMessage.success('已解封')
  await loadSeats()
}

async function rebuildProj() {
  if (!seatTrainId.value || !projDate.value) {
    ElMessage.warning('请选择车次和日期')
    return
  }
  await api.adminRebuildProjection({ train_id: seatTrainId.value, travel_date: projDate.value })
  ElMessage.success('余票投影已重建')
}

async function loadGw() {
  try {
    gw.value = await api.gatewayStats()
  } catch {
    gw.value = {}
  }
}

async function saveQuota() {
  await api.adminUpsertQuota({ ...quotaForm })
  ElMessage.success('配额已保存')
  quotas.value = await api.adminListQuotas()
}

async function delQuota(row) {
  await api.adminDeleteQuota(row.id)
  quotas.value = await api.adminListQuotas()
}

async function saveSale() {
  await api.adminUpsertSalePolicy({ ...saleForm })
  ElMessage.success('放票策略已保存')
  sales.value = await api.adminListSalePolicies()
}

async function saveRisk() {
  await api.adminUpsertRiskBlock({ ...riskForm })
  ElMessage.success('已加入黑名单')
  risks.value = await api.adminListRiskBlocks()
}

async function delRisk(row) {
  await api.adminDeleteRiskBlock(row.id)
  risks.value = await api.adminListRiskBlocks()
}

onMounted(reload)
</script>

<style scoped>
.hdr { display: flex; justify-content: space-between; align-items: center; }
.hint { margin-left: 6px; color: #999; font-size: 12px; }
</style>
