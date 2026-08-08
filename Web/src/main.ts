import 'vuetify/styles'
import '@mdi/font/css/materialdesignicons.css'
import { createApp } from 'vue'
import { createVuetify } from 'vuetify'
import { zhHans } from 'vuetify/locale'

import App from './App.vue'
import router from './router'

// Vuetify 4：全局中文（zhHans），日期组件按中国习惯展示简体中文
const vuetify = createVuetify({
  locale: {
    locale: 'zhHans',
    fallback: 'en',
    messages: { zhHans },
  },
  theme: {
    defaultTheme: 'dark',
  },
})

createApp(App).use(vuetify).use(router).mount('#app')
