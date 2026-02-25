import type { FC } from 'react'
import type { PageKey } from './Toolbar'
import styles from './Sidebar.module.css'

const NAV_ITEMS: { key: PageKey; label: string; icon: string }[] = [
  { key: 'klines',    label: 'K 线图',  icon: '📈' },
  { key: 'feargreed', label: '贪婪指数', icon: '😨' },
]

interface Props {
  page:         PageKey
  onPageChange: (p: PageKey) => void
}

const Sidebar: FC<Props> = ({ page, onPageChange }) => {
  return (
    <aside className={styles.sidebar}>
      <nav className={styles.nav}>
        {NAV_ITEMS.map(item => (
          <button
            key={item.key}
            className={`${styles.navItem} ${page === item.key ? styles.navItemActive : ''}`}
            onClick={() => onPageChange(item.key)}
          >
            <span className={styles.icon}>{item.icon}</span>
            <span className={styles.label}>{item.label}</span>
          </button>
        ))}
      </nav>
    </aside>
  )
}

export default Sidebar
