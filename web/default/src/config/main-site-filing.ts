/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

/**
 * Main-site-only legal filing configuration.
 *
 * IMPORTANT: Never move these records into shared defaults, status responses,
 * seed data, or sub-site provisioning. A copied or newly created sub-site must
 * not inherit the main site's filing identity. Hostname gating is intentional:
 * the same image can be deployed to sub-sites without exposing these records.
 */
const MAIN_SITE_HOSTS = new Set(['api.o1key.cn', 'api.o1key.com'])

export const MAIN_SITE_FILING_LINKS = [
  {
    text: '粤ICP备2026055881号-1',
    href: 'https://beian.miit.gov.cn',
  },
  {
    text: '粤公网安备粤ICP备2026055881号-1',
    href: 'https://www.beian.gov.cn',
  },
] as const

export function shouldShowMainSiteFiling(hostname: string): boolean {
  return MAIN_SITE_HOSTS.has(hostname.trim().toLowerCase())
}
