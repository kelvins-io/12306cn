<template>
  <div class="page">
    <el-card shadow="never">
      <template #header>
        <div class="hdr">
          <span>候补订单</span>
          <el-button @click="load" :loading="loading">刷新</el-button>
        </div>
      </template>
      <el-alert
        title="有人退票后，系统会按候补顺序自动尝试兑现并完成支付（演示环境）。"
        type="info"
        :closable="false"
        style="margin-bottom: 12px"
      />
      <el-table :data="list" v-loading="loading">
        <el-table-column prop="train_no" label="车次" width="100" />
        <el-table-column label="行程" min-width="200">
          <template #default="{ row }">
            {{ row.travel_date }}　{{ row.from_station }} → {{ row.to_station }}
          </template>
        </el-table-column>
        <el-table-column prop="seat_type" label="席别" width="100" />
        <el-table-column prop="passenger_name" label="乘车人" width="100" />
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-tag :type="typeOf(row.status)">{{ textOf(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="100">
          <template #default="{ row }">
            <el-button v-if="row.status === 'pending'" type="danger" link @click="cancel(row)">取消</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import * as api from '../api'

const list = ref([])
const loading = ref(false)

function textOf(s) {
  return { pending: '排队中', fulfilled: '已兑现', cancelled: '已取消', expired: '已过期' }[s] || s
}
function typeOf(s) {
  return { pending: 'warning', fulfilled: 'success', cancelled: 'info', expired: 'info' }[s] || ''
}

async function load() {
  loading.value = true
  try {
    list.value = await api.listWaitlist()
  } finally {
    loading.value = false
  }
}

async function cancel(row) {
  await ElMessageBox.confirm('取消该候补？', '提示')
  await api.cancelWaitlist(row.id)
  ElMessage.success('已取消')
  load()
}

onMounted(load)
</script>

<style scoped>
.hdr {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
</style>
