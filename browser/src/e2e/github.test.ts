import getFreePort from 'get-port'
import { startCase } from 'lodash'
import { appendFile, exists, mkdir, readFile } from 'mz/fs'
import * as path from 'path'
import puppeteer from 'puppeteer'
import puppeteerFirefox from 'puppeteer-firefox'
import webExt from 'web-ext'
import * as util from 'util'
import { saveScreenshotsUponFailures } from '../../../shared/src/e2e/screenshotReporter'

const BROWSER = process.env.E2E_BROWSER || 'chrome'

async function getTokenWithSelector(
    page: puppeteer.Page,
    token: string,
    selector: string
): Promise<puppeteer.ElementHandle> {
    const elements = await page.$$(selector)

    let element: puppeteer.ElementHandle | undefined
    for (const elem of elements) {
        const text = await page.evaluate(element => element.textContent, elem)
        if (text === token) {
            element = elem
            break
        }
    }

    if (!element) {
        throw new Error(`Unable to find token '${token}' with selector ${selector}`)
    }

    return element
}

async function clickElement(page: puppeteer.Page, element: puppeteer.ElementHandle): Promise<void> {
    // Wait for JS to be evaluated (https://github.com/GoogleChrome/puppeteer/issues/1805#issuecomment-357999249).
    await page.waitFor(500)
    await element.click()
}

// Copied from node_modules/puppeteer-firefox/misc/install-preferences.js
async function getFirefoxCfgPath(): Promise<string> {
    const firefoxFolder = path.dirname(puppeteerFirefox.executablePath())
    let configPath: string
    if (process.platform === 'darwin') {
        configPath = path.join(firefoxFolder, '..', 'Resources')
    } else if (process.platform === 'linux') {
        if (!(await exists(path.join(firefoxFolder, 'browser', 'defaults')))) {
            await mkdir(path.join(firefoxFolder, 'browser', 'defaults'))
        }
        if (!(await exists(path.join(firefoxFolder, 'browser', 'defaults', 'preferences')))) {
            await mkdir(path.join(firefoxFolder, 'browser', 'defaults', 'preferences'))
        }
        configPath = firefoxFolder
    } else if (process.platform === 'win32') {
        configPath = firefoxFolder
    } else {
        throw new Error('Unsupported platform: ' + process.platform)
    }
    return path.join(configPath, 'puppeteer.cfg')
}

describe(`Sourcegraph ${startCase(BROWSER)} extension`, () => {
    let browser: puppeteer.Browser
    let page: puppeteer.Page

    // Open browser.
    before(async function() {
        this.timeout(90 * 1000)

        if (BROWSER === 'chrome') {
            const chromeExtensionPath = path.resolve(__dirname, '..', '..', 'build', 'chrome')
            let args: string[] = [
                `--disable-extensions-except=${chromeExtensionPath}`,
                `--load-extension=${chromeExtensionPath}`,
            ]
            if (process.getuid() === 0) {
                // TODO don't run as root in CI
                console.warn('Running as root, disabling sandbox')
                args = [...args, '--no-sandbox', '--disable-setuid-sandbox']
            }
            browser = await puppeteer.launch({ args, headless: false })
        } else {
            // Make sure CSP is disabled in FF preferences,
            // because Puppeteer uses new Function() to evaluate code
            // which is not allowed by the github.com CSP.
            const cfgPath = await getFirefoxCfgPath()
            const disableCspPreference = '\npref("security.csp.enable", false);\n'
            if (!(await readFile(cfgPath, 'utf-8')).includes(disableCspPreference)) {
                await appendFile(cfgPath, disableCspPreference)
            }

            const cdpPort = await getFreePort()
            const firefoxExtensionPath = path.resolve(__dirname, '..', '..', 'build', 'firefox')
            // webExt.util.logger.consoleStream.makeVerbose()
            await webExt.cmd.run(
                {
                    sourceDir: firefoxExtensionPath,
                    firefox: puppeteerFirefox.executablePath(),
                    args: [`-juggler=${cdpPort}`, '-headless'],
                },
                { shouldExitProgram: false }
            )
            const browserWSEndpoint = `ws://127.0.0.1:${cdpPort}`
            browser = await puppeteerFirefox.connect({ browserWSEndpoint })
        }
    })

    beforeEach(async () => {
        page = await browser.newPage()
        page.on('console', message => {
            if (message.text().includes('Download the React DevTools')) {
                return
            }
            if (message.text().includes('[HMR]') || message.text().includes('[WDS]')) {
                return
            }
            console.log('Browser console:', util.inspect(message, { colors: true, depth: 2, breakLength: Infinity }))
        })
    })

    // Take a screenshot when a test fails.
    saveScreenshotsUponFailures(() => page)

    // Close browser.
    after(async () => {
        if (browser) {
            if (page && !page.isClosed()) {
                await page.close()
            }
            await browser.close()
        }
    })

    const repoBaseURL = 'https://github.com/gorilla/mux'

    it('injects View on Sourcegraph', async () => {
        await page.goto(repoBaseURL)
        await page.waitForSelector('li#open-on-sourcegraph')
    })

    it('injects toolbar for code views', async () => {
        await page.goto('https://github.com/gorilla/mux/blob/master/mux.go')
        await page.waitForSelector('.code-view-toolbar')
    })

    it('provides tooltips for single file', async () => {
        await page.goto('https://github.com/gorilla/mux/blob/master/mux.go')

        await page.waitForSelector('.code-view-toolbar')
        const element = await getTokenWithSelector(page, 'NewRouter', 'span.pl-en')

        await clickElement(page, element)

        await page.waitForSelector('.e2e-tooltip-go-to-definition')
    })

    const tokens = {
        base: { text: 'matchHost', selector: 'span.pl-s1' },
        head: { text: 'typ', selector: 'span.pl-s1' },
    }

    for (const diffType of ['unified', 'split']) {
        for (const side of ['base', 'head'] as const) {
            it(`provides tooltips for diff files (${diffType}, ${side})`, async () => {
                await page.goto(`https://github.com/gorilla/mux/pull/328/files?diff=${diffType}`)

                const token = tokens[side]
                const element = await getTokenWithSelector(page, token.text, token.selector)

                // Scrolls the element into view so that code view is in view.
                await element.hover()
                await page.waitForSelector('[data-path="regexp.go"] .code-view-toolbar .open-on-sourcegraph')
                await clickElement(page, element)
                await page.waitForSelector('.e2e-tooltip-go-to-definition')
            })
        }
    }
})
