const $=s=>document.querySelector(s), sections=$('#sections'), empty=$('#empty');
const DESIGN_W=1920, DESIGN_H=1080;
let sectionMap={}, running=false, fitObserver;

function esc(s){return String(s??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]))}
function pickMetric(metrics,name){return(metrics||[]).find(m=>m.name===name)||{}}
function metricText(metrics,name,fallback='—'){const m=pickMetric(metrics,name);return m.text||(m.value!=null?`${Number(m.value).toLocaleString(undefined,{maximumFractionDigits:1})} ${m.unit||''}`.trim():fallback)}
function fmtMetric(m){return m.text||(m.value!=null?`${Number(m.value).toLocaleString(undefined,{maximumFractionDigits:1})} ${m.unit||''}`.trim():'—')}

function maskIP(ip){
  const s=String(ip||'').trim();
  if(!s||s==='—')return'—';
  if(s.includes('*'))return s;
  const v4=s.match(/^(\d{1,3}\.\d{1,3}\.)\d{1,3}\.\d{1,3}$/);
  if(v4)return v4[1]+'*.*';
  const candidate=s.replace(/^\[|\](?::\d+)?$/g,'').split('%')[0];
  if(candidate.includes(':')){
    const head=candidate.split('::',1)[0].split(':').filter(Boolean);
    const tail=candidate.includes('::')?candidate.split('::',2)[1].split(':').filter(Boolean):[];
    const parts=[...head,...tail];
    if(parts.length){
      const first=(head[0]||'0').toLowerCase();
      const second=(head[1]||(head.length<2&&tail.length===7?tail[0]:'0')).toLowerCase();
      return `${first}:${second}:*:*:*:*:*:*`;
    }
  }
  return s;
}

function maskSTUNMapping(udp){
  const text=String(udp?.stunText||'').trim();
  if(!text)return'—';
  const suffix=text.match(/\s*·.*$/)?.[0]||'';
  const mappedIP=String(udp?.mappedIp||'').trim();
  if(mappedIP){
    const masked=maskIP(mappedIP);
    return `${masked.includes(':')?`[${masked}]`:masked}:*****${suffix}`;
  }
  const address=text.match(/^(.+):\d+(\s*·.*)?$/);
  if(address){
    const masked=maskIP(address[1].replace(/^\[|\]$/g,''));
    return `${masked.includes(':')?`[${masked}]`:masked}:*****${address[2]||''}`;
  }
  return text;
}

function maskHostname(name){
  const s=String(name||'').trim();
  if(!s||s==='—')return'—';
  if(s.endsWith('****'))return s;
  return s.slice(0,2)+'****';
}

function latencyClass(ms,text){
  const t=String(text||'');
  if(/超时|timeout/i.test(t))return'latency-timeout';
  if(!ms&&ms!==0)return'latency-unknown';
  if(ms<=50)return'latency-excellent';
  if(ms<=100)return'latency-good';
  if(ms<=200)return'latency-fair';
  if(ms<=250)return'latency-medium';
  return'latency-high';
}

function coloredMs(ms,text){
  const t=text??(ms!=null&&!Number.isNaN(ms)?`${Number(ms).toLocaleString(undefined,{maximumFractionDigits:1})} ms`:'—');
  const cls=latencyClass(Number(ms)||0,t);
  return `<span class="latency ${cls}">${esc(t)}</span>`;
}

function lossClass(pct){
  if(pct==null||pct<0)return'loss-unknown';
  if(pct===0)return'loss-good';
  if(pct<=5)return'loss-medium';
  if(pct<=20)return'loss-high';
  return'loss-bad';
}

function lossDot(pct){
  if(pct==null||pct<0)return'';
  const cls=lossClass(pct);
  const title=pct===0?'丢包 0%':`丢包 ${Number(pct).toLocaleString(undefined,{maximumFractionDigits:0})}%`;
  return `<i class="prov-loss-dot ${cls}" title="${esc(title)}"></i>`;
}

function cellLossPct(c){
  if(!c)return null;
  if(c.loss!=null&&c.loss>=0)return Number(c.loss);
  const m=String(c.lossText||'').match(/丢(\d+(?:\.\d+)?)%/);
  if(m)return Number(m[1]);
  if(/超时|不可达/i.test(c.text||''))return 100;
  return null;
}

function provinceAvgLoss(cells){
  const vals=Object.values(cells||{}).map(cellLossPct).filter(v=>v!=null);
  if(!vals.length)return null;
  return vals.reduce((a,b)=>a+b,0)/vals.length;
}

function colorizeTextMs(text){
  const s=String(text??'');
  if(!s)return'—';
  if(/超时|timeout/i.test(s))return coloredMs(0,s);
  return esc(s).replace(/(\d+(?:\.\d+)?)\s*ms/gi,(m,num)=>`<span class="latency ${latencyClass(Number(num),m)}">${esc(m)}</span>`);
}

function kvTable(rows){return `<table class="kv-table">${rows.filter(r=>r[1]!=null&&r[1]!==''&&r[1]!=='—').map(r=>`<tr><th>${esc(r[0])}</th><td>${r[2]?r[1]:esc(r[1])}</td></tr>`).join('')}</table>`}
function kvTableHtml(rows,extraClass=''){
  const cls=extraClass?`kv-table ${extraClass}`:'kv-table';
  const cols=extraClass?'<colgroup><col class="ip-matrix-label-col"><col></colgroup>':'';
  return `<table class="${cls}">${cols}${rows.filter(r=>r[1]!=null&&r[1]!==''&&r[1]!=='—').map(r=>`<tr><th>${esc(r[0])}</th><td>${r[1]}</td></tr>`).join('')}</table>`;
}
function flagBadge(name,ok){return `<span class="hw-flag ${ok?'hw-flag-ok':'hw-flag-bad'}">${ok?'✓':'✗'} ${esc(name)}</span>`}
function hwScoreCell(score){
  const n=score??0;
  return `<span class="speed-value">${esc(String(n))}</span>`;
}
function hwScoreRow(label,score){
  return `<div class="svc-item"><span class="svc-name">${esc(label)}</span><span class="svc-status">${hwScoreCell(score)}</span></div>`;
}
function hwScoreGridHtml(total,cpuScore,memScore,diskScore){
  const rows=[['总分',total],['CPU',cpuScore],['内存',memScore],['磁盘',diskScore]];
  return svcGridHtml(rows.map(([label,val])=>hwScoreRow(label,val)));
}

function ipTag(text,kind='muted'){return `<span class="ip-tag ip-tag-${kind}">${esc(text)}</span>`}
function boolCell(val){
  const s=String(val??'').trim().toLowerCase();
  if(s==='true')return ipTag('true','ok');
  if(s==='false')return ipTag('false','bad');
  return null;
}
function yesNoCell(val){
  const s=String(val??'');
  if(s==='是')return ipTag('是','bad');
  if(s==='否')return ipTag('否','ok');
  if(!s||/无数据|未知|无|—/.test(s))return ipTag('—','muted');
  return ipTag(s,'muted');
}
function purityCell(purity,metricTextVal){
  let pct=null,level='';
  if(purity&&typeof purity==='object'){
    if(purity.percent!=null&&purity.percent!=='')pct=Number(purity.percent);
    level=String(purity.level||'');
  }
  if(pct==null||Number.isNaN(pct)){
    const m=String(metricTextVal||'').match(/(\d+(?:\.\d+)?)\s*%?/);
    if(m)pct=Number(m[1]);
  }
  if(pct==null||Number.isNaN(pct))return '';
  pct=Math.round(pct);
  if(!level)level=pct>=80?'good':pct>=40?'warn':'bad';
  const kind=level==='good'?'ok':level==='warn'?'warn':'bad';
  return ipTag(pct+'%',kind);
}
function stealCell(text){
  const s=String(text||'');
  const m=s.match(/^([\d.]+)%/);
  if(!m)return esc(s);
  const pct=Number(m[1]);
  let kind='ok';
  if(pct>=15)kind='bad';
  else if(pct>=5)kind='warn';
  else if(pct>=1)kind='muted';
  return ipTag(s,kind);
}
function enabledCell(text){
  const s=String(text||'');
  if(s==='已启用')return ipTag(s,'warn');
  if(s==='未启用')return ipTag(s,'ok');
  return yesNoCell(s);
}
function cgroupStatusCell(text){
  const s=String(text||'');
  if(/可能缩水|低于宿主机|已限速/.test(s))return ipTag(s,'bad');
  if(/已限额|有限额/.test(s))return ipTag(s,'warn');
  if(/正常|无限制|无硬限制/.test(s))return ipTag(s,'ok');
  return ipTag(s||'—','muted');
}
function statusCell(val){
  const s=String(val??'').trim();
  if(/不可用|已标记|封禁|失败|阻断/.test(s))return ipTag(s,'bad');
  if(/自制剧|仅网页|仅APP/.test(s))return ipTag(s,'warn');
  if(/^可用$|^正常$|解锁/.test(s)||(/可用/.test(s)&&!/不可用/.test(s)))return ipTag(s,'ok');
  if(/查询失败|查询受限|远端不可达/.test(s))return ipTag(s,'warn');
  if(s==='是')return ipTag('是','bad');
  if(s==='否')return ipTag('否','ok');
  return ipTag(s||'—','muted');
}
function ipCategoryFlags(categories){
  return (categories||[]).map(c=>{
    const on=!!c.active;
    const kind=on?(c.kind||'ok'):'muted';
    return `<span class="hw-flag ip-cat ${on?'ip-cat-on ip-cat-'+kind:'ip-cat-off'}">${on?'✓':'○'} ${esc(c.label)}</span>`;
  }).join('');
}
function cpuName(cpu){
  const spec=(cpu.details||{}).spec||{};
  if(spec.model)return spec.model;
  return String(metricText(cpu.metrics,'型号',cpu.summary)).split(' · ')[0]||'—';
}
function cpuSpecLine(cpu){
  const spec=(cpu.details||{}).spec||{};
  const parts=[];
  if(spec.physicalCores)parts.push(`${spec.physicalCores} 物理核`);
  else if(pickMetric(cpu.metrics,'物理核心').text)parts.push(pickMetric(cpu.metrics,'物理核心').text);
  if(spec.logicalCores)parts.push(`${spec.logicalCores} 线程`);
  else if(pickMetric(cpu.metrics,'逻辑线程').text)parts.push(pickMetric(cpu.metrics,'逻辑线程').text);
  const mhz=spec.mhz||String(metricText(cpu.metrics,'频率','')).replace(/\s*MHz$/,'');
  if(mhz&&mhz!=='未知')parts.push(`${mhz} MHz`);
  const util=spec.utilization||metricText(cpu.metrics,'利用率','');
  if(util&&util!=='—')parts.push(`利用率 ${util}`);
  if(!parts.length){
    const legacy=metricText(cpu.metrics,'型号',cpu.summary);
    const tail=legacy.includes(' · ')?legacy.split(' · ').slice(1).join(' · '):'';
    return tail||'—';
  }
  return parts.join(' · ');
}
function fallbackIPCategories(sources,ipMetrics){
  const votes={};
  const bump=(k,n=1)=>{votes[k]=(votes[k]||0)+n;};
  const map={机房:'机房IP',ISP:'家宽IP','移动 ISP':'移动IP',教育:'教育IP',商业:'商业IP',政府:'政府IP',CDN:'CDN',爬虫:'爬虫IP',组织:'组织IP'};
  (sources||[]).forEach(s=>{[s.type,s.companyType].forEach(t=>{if(map[t])bump(map[t]);});});
  if(metricText(ipMetrics,'机房')==='是')bump('机房IP',2);
  if(metricText(ipMetrics,'代理')==='是')bump('代理IP',2);
  if(votes['机房IP']&&votes['家宽IP']){
    if(votes['机房IP']>=votes['家宽IP'])delete votes['家宽IP']; else delete votes['机房IP'];
  }
  const labels=[['家宽IP','ok'],['机房IP','warn'],['移动IP','ok'],['教育IP','ok'],['商业IP','muted'],['政府IP','muted'],['CDN','warn'],['代理IP','bad'],['爬虫IP','bad'],['组织IP','muted']];
  return labels.map(([label,kind])=>({label,active:!!votes[label],kind}));
}
function mailCompact(mail){
  const rows=mail.results||[];
  if(!rows.length)return'';
  const ok=rows.filter(r=>r.status==='可用').length;
  const chips=rows.map(r=>{
    const kind=r.status==='可用'?'ok':r.status==='不可用'?'bad':'warn';
    const label=r.provider||shortMailTarget(r.target);
    const tip=[r.target,r.status].filter(Boolean).join(' · ');
    return `<span class="ip-tag ip-tag-${kind}"${tip?` title="${esc(tip)}"`:''}>${esc(label)}</span>`;
  }).join('');
  return `<div class="ip-mail-row"><span>邮件 ${ok}/${rows.length}</span>${statusCell(ok===rows.length?'可用':'不可用')}</div><div class="mail-grid">${chips}</div>`;
}
function blacklistCompact(black){
  const listedCount=black.listed??((black.results||[]).filter(r=>r.status==='已标记').length);
  const blSummary=[
    ipTag(`有效 ${black.valid??0}`,'muted'),
    ipTag(`正常 ${black.normal??0}`,'ok'),
    ipTag(`标记 ${listedCount}`,listedCount>0?'bad':'ok')
  ].join(' ');
  const overall=listedCount>0?ipTag(`${listedCount} 已标记`,'bad'):ipTag('未标记','ok');
  return `<div class="ip-mail-row"><span>DNSBL</span>${overall}</div><div class="ip-mail-tags">${blSummary}</div>`;
}
function hostingCell(type){
  const s=String(type||'').trim();
  if(!s||s==='无数据'||s==='—')return ipTag('—','muted');
  if(/机房/.test(s)&&!/非机房/.test(s))return ipTag(s,'warn');
  return ipTag(s,'muted');
}
function svcStatusLabel(status){
  const s=String(status||'').trim();
  if(/解锁|封禁|自制剧|仅网页|仅APP|失败/.test(s))return s.replace(/\s.*/,'');
  return s||'—';
}
function svcToneClass(status){
  const s=String(status||'').trim();
  if(/封禁|失败|不可用|阻断/.test(s))return'svc-bad';
  if(/自制剧|仅网页|仅APP/.test(s))return'svc-warn';
  if(/解锁/.test(s))return'svc-ok';
  return'';
}
function svcLegendHtml(){
  return '<div class="svc-legend latency-legend"><div class="latency-legend-row"><span>状态</span><i class="svc-legend-ok"></i>解锁<i class="svc-legend-warn"></i>自制剧/仅网页/仅APP<i class="svc-legend-bad"></i>封禁</div></div>';
}
function svcSectionHtml(svc){
  const items=(svc.details?.items)||[];
  const metrics=svc.metrics||[];
  if(!items.length&&!metrics.length)return'';
  const legend=svcLegendHtml();
  const svcItems=items.length?items.map(it=>{
    const meta=[it.region?`[${it.region}]`:null,it.unlockType].filter(Boolean).join(' ');
    return svcRow(it.name,it.status||'—',meta);
  }).join(''):metrics.map(m=>svcRow(m.name,svcStatusFromText(m.text),svcMetaFromText(m.text))).join('');
  const svcCount=items.length||metrics.length;
  if(!svcItems)return'';
  return `<h3>流媒体与 AI</h3><div class="svc-grid${svcCount>14?' svc-grid-dense':''}">${svcItems}</div>${legend}`;
}
function svcRow(name,status,meta=''){
  const tone=svcToneClass(status);
  const metaHtml=meta?`<span class="svc-meta">${esc(meta)}</span>`:'';
  return `<div class="svc-item${tone?' '+tone:''}"><span class="svc-name">${esc(name)}</span>${metaHtml}<span class="svc-status">${statusCell(svcStatusLabel(status))}</span></div>`;
}
function svcGridHtml(items,denseThreshold=14){
  if(!items.length)return'';
  const dense=items.length>denseThreshold?' svc-grid-dense':'';
  return `<div class="svc-grid${dense}">${items.join('')}</div>`;
}
function intlPingRow(city,ms,text){
  const val=ms!=null&&ms>0?coloredMs(ms,text):colorizeTextMs(text||'—');
  return `<div class="svc-item"><span class="svc-name">${esc(city)}</span><span class="svc-status">${val}</span></div>`;
}
function svcStatusFromText(text){
  const s=String(text||'');
  if(/封禁|失败/.test(s))return'封禁';
  if(/自制剧/.test(s))return'自制剧';
  if(/仅网页/.test(s))return'仅网页';
  if(/仅APP/.test(s))return'仅APP';
  if(/解锁/.test(s))return'解锁';
  return s.trim()||'—';
}
function svcMetaFromText(text){
  const s=String(text||'').trim();
  const m=s.match(/^(?:解锁|封禁|自制剧|仅网页|仅APP)\s*(.*)$/);
  return m&&m[1]?m[1].trim():'';
}
function shortMailTarget(t){
  const s=String(t||'').toLowerCase();
  if(s.includes('google'))return'Gmail';
  if(s.includes('outlook'))return'Outlook';
  if(s.includes('qq.com'))return'QQ';
  return String(t||'').split(':')[0];
}
function ipMatrix(headers,rows,extraClass=''){
  if(!rows.length)return'';
  const tableCls=extraClass?`ip-matrix ${extraClass}`:'ip-matrix';
  return `<table class="${tableCls}"><colgroup><col class="ip-matrix-label-col">${headers.map(()=>'<col>').join('')}</colgroup><thead><tr><th></th>${headers.map(h=>`<th>${esc(h)}</th>`).join('')}</tr></thead><tbody>${rows.map(r=>`<tr><th>${esc(r[0])}</th>${r.slice(1).map(c=>`<td>${c}</td>`).join('')}</tr>`).join('')}</tbody></table>`;
}
function riskScoreCell(value){
  const s=String(value??'');
  const m=s.match(/^(\d+(?:\.\d+)?)/);
  if(!m)return ipTag(s||'—','muted');
  const n=Number(m[1]);
  const lvl=n>=75?'bad':n>=45?'warn':n>=20?'medium':'ok';
  const tagLvl=lvl==='medium'?'warn':lvl;
  const tail=s.slice(m[0].length).replace(/^\s*\|?\s*/,'').trim();
  const text=tail?`${m[1]} · ${tail}`:m[1];
  return ipTag(text,tagLvl);
}

function riskScoreTable(riskScores){
  const entries=Object.entries(riskScores).filter(([k])=>k!=='提示');
  if(!entries.length)return'';
  const heads=entries.map(([k])=>`<th>${esc(k)}</th>`).join('');
  const cells=entries.map(([,v])=>`<td>${riskScoreCell(v)}</td>`).join('');
  return `<table class="ip-risk-score"><thead><tr>${heads}</tr></thead><tbody><tr>${cells}</tr></tbody></table>`;
}

const COUNTRY_ALIASES={us:'US',usa:'US','united states':'US','美国':'US',hk:'HK','hong kong':'HK','香港':'HK',gb:'GB','united kingdom':'GB','英国':'GB',jp:'JP',japan:'JP','日本':'JP',cn:'CN',china:'CN','中国':'CN',tw:'TW',taiwan:'TW',sg:'SG',singapore:'SG',de:'DE',germany:'DE',fr:'FR',france:'FR',ca:'CA',canada:'CA',au:'AU',australia:'AU',kr:'KR',korea:'KR','south korea':'KR',nl:'NL',netherlands:'NL',in:'IN',india:'IN'};
const COUNTRY_CN=window.COUNTRY_CN||{};
function formatCountryCode(v){
  const s=String(v??'').trim();
  if(!s||s==='—')return'—';
  const boxed=s.match(/^\[([A-Za-z]{2,3})\]\s*(.+)$/);
  if(boxed){
    const code=boxed[1].toUpperCase();
    const tail=boxed[2].trim();
    if(/[\u4e00-\u9fff]/.test(tail))return `[${code}] ${tail}`;
    const cn=COUNTRY_CN[code];
    return cn?`[${code}] ${cn}`:`[${code}] ${tail}`;
  }
  const codeOnly=s.match(/^\[([A-Za-z]{2,3})\]$/);
  if(codeOnly){
    const code=codeOnly[1].toUpperCase();
    return COUNTRY_CN[code]?`[${code}] ${COUNTRY_CN[code]}`:`[${code}]`;
  }
  const low=s.toLowerCase();
  if(COUNTRY_ALIASES[low]){
    const code=COUNTRY_ALIASES[low];
    const cn=COUNTRY_CN[code]||(/[\u4e00-\u9fff]/.test(s)?s:'');
    return cn?`[${code}] ${cn}`:`[${code}]`;
  }
  for(const [k,c] of Object.entries(COUNTRY_ALIASES)){
    if(low.includes(k)){
      const cn=COUNTRY_CN[c]||(/[\u4e00-\u9fff]/.test(s)?s:'');
      return cn?`[${c}] ${cn}`:`[${c}]`;
    }
  }
  if(/^[A-Za-z]{2,3}$/.test(s)){
    const code=s.toUpperCase();
    return COUNTRY_CN[code]?`[${code}] ${COUNTRY_CN[code]}`:`[${code}]`;
  }
  return s;
}
function countryCell(v){return esc(formatCountryCode(v));}

function ipColumn(){
  const ip=sectionMap['ip-quality']||{},svc=sectionMap.services||{};
  const d=ip.details||{},b=d.base||{},black=d.blacklist||{},mail=d.mail||{};
  const sources=d.sources||[],riskFactors=d.riskFactors||{},riskScores=d.riskScores||{};
  const ipVal=maskIP(b.ip||pickMetric(ip.metrics,'公网 IP').text||ip.summary?.split('·')[0]?.trim());
  const asnOrg=[b.asn||metricText(ip.metrics,'自治系统'),b.organization||metricText(ip.metrics,'组织')].filter(Boolean).join(' · ');

  const ipCategories=(b.ipCategories&&b.ipCategories.length)?b.ipCategories:fallbackIPCategories(sources,ip.metrics);
  const purityHtml=purityCell(d.ipPurity,metricText(ip.metrics,'IP纯净度'));

  const basicRows=[
    ['IP',`<strong class="ip-highlight">${esc(ipVal)}</strong>`,true],
    ['AS/组织',asnOrg||'—',true],
    ['坐标',b.coordinates?`<a class="ip-map" href="${esc(b.map)}" target="_blank" rel="noopener">${esc(b.coordinates)}</a>`:'—',true],
    ['位置',b.city||metricText(ip.metrics,'位置'),true],
    ['使用地区',formatCountryCode(b.country),true],
    ['注册地区',formatCountryCode(b.regCountry),true],
    ['时区',b.timezone||'—',true],
    ['ISP',b.isp||'—',true],
    ['IP类型',b.ipType?(String(b.ipType).includes('广播')?ipTag(b.ipType,'bad'):String(b.ipType).includes('原生')?ipTag(b.ipType,'ok'):ipTag(b.ipType,'muted')):'—',true],
    ...(purityHtml?[['IP纯净度',purityHtml,true]]:[]),
    ['代理',yesNoCell(metricText(ip.metrics,'代理')),true],
    ['机房',yesNoCell(metricText(ip.metrics,'机房')),true]
  ];

  const activeSources=sources.filter(s=>[s.type,s.companyType,s.country,s.city,s.asn,s.org].some(v=>v&&v!=='无数据'&&v!=='—'));
  const srcNames=activeSources.map(s=>s.name||'—');
  const srcMatrixRows=srcNames.length?[
    ['用途类型',...activeSources.map(s=>hostingCell(s.type))],
    ['公司类型',...activeSources.map(s=>hostingCell(s.companyType))],
    ['国家',...activeSources.map(s=>countryCell(s.country))],
    ['城市',...activeSources.map(s=>esc(s.city||'—'))],
    ['ASN',...activeSources.map(s=>esc(s.asn||'—'))],
    ['组织',...activeSources.map(s=>esc(s.org||'—'))]
  ]:[];

  const factorNames=Object.keys(riskFactors);
  const factorSources=[...new Set(factorNames.flatMap(k=>Object.keys(riskFactors[k]||{})))];
  const factorRows=factorNames.map(name=>[name,...factorSources.map(src=>name==='地区'?countryCell((riskFactors[name]||{})[src]):yesNoCell((riskFactors[name]||{})[src]||'—'))]);
  const scoreTable=riskScoreTable(riskScores);

  const mailBlock=mailCompact(mail);
  const blBlock=blacklistCompact(black);

  const svcItems=svcSectionHtml(svc);

  return `<div class="col-panel">
    <h3>基础信息</h3>${kvTableHtml(basicRows,'ip-label-table')}
    ${ipCategories.length?`<h3>IP 用途分类</h3><div class="hw-flags ip-cat-grid">${ipCategoryFlags(ipCategories)}</div>`:''}
    ${srcMatrixRows.length?`<h3>IP 类型属性</h3>${ipMatrix(srcNames,srcMatrixRows)}`:''}
    ${scoreTable?`<h3>风险评分</h3>${scoreTable}`:''}
    ${factorRows.length?`<h3>风险因素</h3>${ipMatrix(factorSources,factorRows)}`:''}
    ${svcItems}
    <h3>邮件与黑名单</h3>
    <div class="ip-mail-row"><span>25端口</span>${statusCell(metricText(ip.metrics,'25端口',mail.localPort25||(mail.outbound25?'可用':'不可用')))}</div>
    ${mailBlock}${blBlock}
  </div>`;
}

function systemColumn(){
  const sys=sectionMap.system||{},cpu=sectionMap.cpu||{},mem=sectionMap.memory||{},disk=sectionMap.disk||{};
  const mb=(sys.details||{}).motherboard||{},flags=(cpu.details||{}).flags||{};
  const flagHtml=Object.entries(flags).map(([k,v])=>flagBadge(k,v)).join('');
  const osRows=[
    ['系统',metricText(sys.metrics,'操作系统',sys.summary),false],
    ['虚拟化',metricText(sys.metrics,'虚拟化'),false],
    ['架构',metricText(sys.metrics,'架构'),false],
    ['核心',metricText(sys.metrics,'处理器核心'),false],
    ['内存',metricText(sys.metrics,'物理内存',metricText(mem.metrics,'内存总量')),false],
    ['运行',metricText(sys.metrics,'运行时长'),false],
    ['负载',metricText(sys.metrics,'系统负载'),false],
    ['时区',metricText(sys.metrics,'区域设置'),false],
    ['容器',metricText(sys.metrics,'容器'),false],
    ['BBR',metricText(sys.metrics,'BBR'),false],
    ['TCP',metricText(sys.metrics,'TCP 拥塞算法'),false],
    ['调度',metricText(sys.metrics,'队列调度算法'),false],
    ['CPU限额',metricText(sys.metrics,'CPU 限额',sys.details?.cgroup?.cpuText),false],
    ['CPU状态',cgroupStatusCell(metricText(sys.metrics,'CPU 状态',sys.details?.cgroup?.cpuStatus)),true],
    ['内存限额',metricText(sys.metrics,'内存限额',sys.details?.cgroup?.memText),false],
    ['内存状态',cgroupStatusCell(metricText(sys.metrics,'内存状态',sys.details?.cgroup?.memStatus)),true],
    ['I/O限额',metricText(sys.metrics,'I/O 限额',sys.details?.cgroup?.ioText),false],
    ['BIOS',mb.bios||'—',false],
    ['芯片组',mb.chipset||'—',false],
    ['网卡',mb.network||'—',false]
  ];
  const perfRows=[
    ['CPU',cpuName(cpu),false],
    ['规格',cpuSpecLine(cpu),false],
    ['偷取时间',stealCell(metricText(cpu.metrics,'偷取时间')),true],
    ['缓存',metricText(cpu.metrics,'缓存',cpu.details?.cache),false],
    ['SHA-256',fmtMetric(pickMetric(cpu.metrics,'SHA-256')),false],
    ['gzip',fmtMetric(pickMetric(cpu.metrics,'gzip')),false],
    ['复制',fmtMetric(pickMetric(mem.metrics,'复制')),false],
    ['延迟',fmtMetric(pickMetric(mem.metrics,'随机延迟')),false],
    ['已用内存',metricText(mem.metrics,'已用内存'),false],
    ['Swap',metricText(mem.metrics,'Swap 已用',metricText(mem.metrics,'Swap 总量')),false],
    ['内存气球',enabledCell(metricText(mem.metrics,'内存气球')),true],
    ['KSM',enabledCell(metricText(mem.metrics,'KSM')),true],
    ['顺序写',fmtMetric(pickMetric(disk.metrics,'顺序写')),false],
    ['顺序读',fmtMetric(pickMetric(disk.metrics,'顺序读')),false],
    ['4K读',fmtMetric(pickMetric(disk.metrics,'4K 随机读')),false],
    ['4K写',fmtMetric(pickMetric(disk.metrics,'4K 随机写')),false],
    ['fsync P50',fmtMetric(pickMetric(disk.metrics,'fsync P50')),false],
    ['fsync P99',fmtMetric(pickMetric(disk.metrics,'fsync P99')),false],
    ['磁盘',metricText(disk.metrics,'测试设备',disk.details?.device),false],
    ['文件系统',metricText(disk.metrics,'文件系统',disk.details?.volume?.fstype),false],
    ['块设备',metricText(disk.metrics,'块设备',disk.details?.volume?.blockDevice),false],
    ['I/O调度',metricText(disk.metrics,'I/O 调度器',disk.details?.volume?.scheduler),false],
    ['总容量',fmtMetric(pickMetric(disk.metrics,'磁盘总容量')),false],
    ['可用',fmtMetric(pickMetric(disk.metrics,'磁盘可用容量')),false],
    ['使用率',fmtMetric(pickMetric(disk.metrics,'磁盘使用率')),false],
    ['Inode',fmtMetric(pickMetric(disk.metrics,'Inode 使用率')),false],
    ['显卡',disk.details?.gpu||'—',false],
    ...(disk.details?.profiles||[]).flatMap(p=>[[p.name+' 读',p.read||'—',false],[p.name+' 写',p.write||'—',false]])
  ];
  const hwTotal=(cpu.score||0)+(mem.score||0)+(disk.score||0);
  return `<div class="col-panel"><h3>系统信息</h3>${kvTable(osRows)}<h3>性能摘要</h3>${kvTable(perfRows)}${flagHtml?`<div class="hw-flags">${flagHtml}</div>`:''}${hwScoreGridHtml(hwTotal,cpu.score,mem.score,disk.score)}</div>`;
}

function metricCell(m){
  const unit=(m.unit||'').toLowerCase();
  if(unit==='ms'||/\bms\b/i.test(m.text||''))return colorizeTextMs(fmtMetric(m));
  const b=boolCell(m.text);
  if(b)return b;
  return esc(fmtMetric(m));
}

function compactHops(hops){
  const list=(hops||[]).map(maskIP).filter(Boolean);
  if(!list.length)return'';
  if(list.length<=4)return list.join(' → ');
  return `${list[0]} → … → ${list[list.length-1]}`;
}

function routeRows(){
  const route=sectionMap.route||{};
  const returns=(route.details||{}).returnRoutes||[];
  if(returns.length)return returns.map(r=>{
    const label=`${r.city}${r.carrier}`;
    const mapHtml=routeMapHtml(r);
    const ping=r.pingMs!=null?coloredMs(r.pingMs,r.pingText):colorizeTextMs(r.pingText||'—');
    return `<tr><th>${esc(label)}</th><td class="route-line">${mapHtml}</td><td class="route-ping">${ping}</td></tr>`;
  }).join('');
  return(route.metrics||[]).map(m=>{
    const parts=String(m.text||'').split('·').map(s=>s.trim());
    const line=routeLineHtml(parts[0]||'—');
    const ping=parts.length>1?colorizeTextMs(parts.slice(1).join(' · ')):colorizeTextMs(m.text);
    return `<tr><th>${esc(m.name)}</th><td class="route-line">${line}</td><td class="route-ping">${ping}</td></tr>`;
  }).join('');
}

function extractRouteRegionFromRest(rest){
  let geo=String(rest||'').trim();
  const bi=geo.indexOf(']');
  if(bi>=0)geo=geo.slice(bi+1).trim();
  else geo=geo.replace(/^AS\d+\s+/,'').trim();
  const di=geo.indexOf('  ');
  if(di>0)geo=geo.slice(0,di).trim();
  const parts=geo.trim().split(/\s+/);
  const country={中国:1,美国:1,日本:1,英国:1,德国:1,法国:1,新加坡:1,香港:1,台湾:1};
  let region='';
  for(const p of parts){
    if(/\./.test(p))break;
    if(country[p]){if(p==='香港'||p==='台湾'||p==='新加坡')region=p;continue;}
    region=p;
  }
  return region;
}

function shouldSkipRouteIP(ip,rest){
  if(!ip||ip==='*'||/RFC1918/i.test(rest||''))return true;
  return /^10\.|^172\.(1[6-9]|2\d|3[01])\.|^192\.168\.|^127\./.test(ip);
}

function parseRouteIPHopsFromOutput(output){
  const out=[];
  const seen=new Set();
  for(const line of String(output||'').split('\n')){
    const m=line.match(/^\s*(\d+)\s+(\S+)\s+(.+)$/);
    if(!m)continue;
    const ip=m[2], rest=m[3].trim();
    if(shouldSkipRouteIP(ip,rest))continue;
    if(seen.has(ip))continue;
    seen.add(ip);
    out.push({ip, region:extractRouteRegionFromRest(rest)});
  }
  return out;
}

function routeIPHops(r){
  const parsed=parseRouteIPHopsFromOutput(r.output);
  if(parsed.length)return parsed;
  return(r.hops||[]).filter(h=>h&&h!=='*'&&!shouldSkipRouteIP(h,'')).map(ip=>({ip, region:''}));
}

function routeIPHopNodeHtml(hop){
  const ip=maskIP(hop.ip);
  const region=String(hop.region||'').trim();
  const text=region?`[${region}]${ip}`:ip;
  return ipTag(text,'muted');
}

function routeIPPathHtml(r){
  const hops=routeIPHops(r);
  if(!hops.length)return'<span class="route-unknown">未知</span>';
  const parts=['<div class="route-map route-ip-map">'];
  hops.forEach((h,i)=>{
    if(i>0)parts.push('<i>→</i>');
    parts.push(routeIPHopNodeHtml(h));
  });
  parts.push('</div>');
  return parts.join('');
}

function routeHopRows(){
  const returns=(sectionMap.route?.details?.returnRoutes)||[];
  if(!returns.length)return'';
  return returns.map(r=>{
    const label=`${r.city}${r.carrier}`;
    const pathHtml=routeIPPathHtml(r);
    return `<tr><th>${esc(label)}</th><td class="route-line">${pathHtml}</td></tr>`;
  }).join('');
}

const ROUTE_LABEL_PATTERNS=[
  {label:'CN2 GIA',re:/CN2GIA|CN2-GIA|AS4809|59\.43\./i},
  {label:'电信 163',re:/AS4134|AS4847|202\.97\.|CHINANET|CHINATELECOM/i},
  {label:'联通 9929',re:/AS9929|CUII/i},
  {label:'联通 4837',re:/AS4837|UNICOM-BACKBONE/i},
  {label:'CMIN2',re:/AS58807|CMIN2/i},
  {label:'CMI',re:/AS58453|CMI\.CHINAMOBILE|CMI-INT/i},
  {label:'移动',re:/AS9808|CMNET/i},
  {label:'HE',re:/AS6939|HURRICANE/i},
  {label:'Cogent',re:/AS174|COGENT/i},
  {label:'NTT',re:/AS2914|NTT/i}
];

function detectRouteLabelText(text){
  const u=String(text||'').toUpperCase();
  for(const p of ROUTE_LABEL_PATTERNS){
    if(p.re.test(u))return p.label;
  }
  const m=String(text||'').match(/\[([^\]]+)\]/);
  if(!m)return'';
  const tag=m[1].toUpperCase();
  if(/CN2/.test(tag))return'CN2 GIA';
  if(/CMI/.test(tag))return'CMI';
  if(/CHINANET|CHINATELECOM/.test(tag))return'电信 163';
  if(/CU|UNICOM/.test(tag))return'联通 4837';
  if(/HURRICANE/.test(tag))return'HE';
  if(/CMNET|CMIN/.test(tag))return'移动';
  return m[1];
}

function parseRoutePathFromOutput(output){
  const out=[];
  let prev='';
  for(const line of String(output||'').split('\n')){
    const m=line.match(/^\s*(\d+)\s+(\S+)\s+(.+)$/);
    if(!m)continue;
    const ip=m[2], rest=m[3].trim();
    if(ip==='*'||/RFC1918/i.test(rest))continue;
    if(/^10\.|^172\.(1[6-9]|2\d|3[01])\.|^192\.168\.|^127\./.test(ip))continue;
    let geo=rest;
    const bi=geo.indexOf(']');
    if(bi>=0)geo=geo.slice(bi+1).trim();
    else geo=geo.replace(/^AS\d+\s+/,'').trim();
    const di=geo.indexOf('  ');
    if(di>0)geo=geo.slice(0,di).trim();
    geo=geo.trim();
    const parts=geo.split(/\s+/);
    const country={中国:1,美国:1,日本:1,英国:1,德国:1,法国:1,新加坡:1,香港:1,台湾:1};
    let region='';
    for(const p of parts){
      if(/\./.test(p))break;
      if(country[p]){if(p==='香港'||p==='台湾'||p==='新加坡')region=p;continue;}
      region=p;
    }
    const label=detectRouteLabelText(rest);
    if(!label)continue;
    const key=`${region}|${label}`;
    if(key===prev)continue;
    prev=key;
    out.push({region,label});
    if(out.length>=12)break;
  }
  return out;
}

function routePathNodes(r){
  if((r.path||[]).length)return r.path;
  const parsed=parseRoutePathFromOutput(r.output);
  if(parsed.length)return parsed;
  return(r.labels||[]).map(l=>({region:'',label:l}));
}

function routeLabelKind(label){
  const l=String(label||'');
  if(/CN2\s*GIA/i.test(l))return'premium';
  if(/9929/i.test(l))return'premium';
  if(/CMIN2/i.test(l))return'premium';
  if(/电信|163/i.test(l))return'telecom';
  if(/联通|4837/i.test(l))return'unicom';
  if(/^CMI$/i.test(l)||/\bCMI\b/i.test(l))return'cmi';
  if(/移动|CMNET/i.test(l))return'mobile';
  if(/HE|HURRICANE|Cogent|NTT/i.test(l))return'transit';
  return'muted';
}

function routeTag(text){return ipTag(text,routeLabelKind(text))}

function routeNodeHtml(node){
  const label=String(node.label||'').trim();
  const region=String(node.region||'').trim();
  const text=region?`[${region}]${label}`:label;
  return ipTag(text,routeLabelKind(label));
}

function routeMapHtml(r){
  const nodes=routePathNodes(r);
  if(!nodes.length)return'<span class="route-unknown">未知</span>';
  const parts=['<div class="route-map">',ipTag('本机','origin')];
  nodes.forEach((n,i)=>{
    parts.push('<i>→</i>');
    parts.push(routeNodeHtml(n));
  });
  parts.push('</div>');
  return parts.join('');
}

function routeLineHtml(line){
  const s=String(line||'').trim();
  if(!s||s==='—'||s==='Unknown')return'<span class="route-unknown">未知</span>';
  const parts=s.split(/\s*→\s*/).map(p=>p.trim()).filter(Boolean);
  if(!parts.length)return esc(s);
  return parts.map(p=>routeTag(p)).join('<span class="route-arrow"> → </span>');
}

function latencyLegendHtml(){
  return '<div class="latency-legend"><div class="latency-legend-row"><span>延迟</span><i class="latency-excellent"></i>≤50ms<i class="latency-good"></i>51-100ms<i class="latency-fair"></i>101-200ms<i class="latency-medium"></i>201-250ms<i class="latency-high"></i>&gt;250ms<i class="latency-timeout"></i>超时</div><div class="latency-legend-row"><span>丢包</span><i class="loss-good"></i>0%<i class="loss-medium"></i>≤5%<i class="loss-high"></i>≤20%<i class="loss-bad"></i>&gt;20%</div></div>';
}

function pingGridItems(items){
  if(!items.length)return'';
  return `<div class="ping-grid">${items.map(it=>{
    const label=esc(it.label||'—');
    const val=it.ms!=null&&it.ms>0?coloredMs(it.ms,it.text):colorizeTextMs(it.text||'—');
    return `<div class="ping-item"><small>${label}</small>${val}</div>`;
  }).join('')}</div>`;
}

function chinaPingGrid(){
  const routes=(sectionMap.route?.details?.returnRoutes)||[];
  if(!routes.length)return'';
  const cities=['北京','上海','广州'];
  const carriers=['电信','联通','移动'];
  const rows=cities.map(city=>[city,...carriers.map(carrier=>{
    const r=routes.find(x=>x.city===city&&x.carrier===carrier);
    if(r?.pingMs!=null&&r.pingMs>0)return coloredMs(r.pingMs,r.pingText);
    return colorizeTextMs(r?.pingText||'—');
  })]);
  return ipMatrix(carriers,rows,'ping-matrix');
}

function chinaLatencyList(){
  return (sectionMap['china-latency']?.details?.chinaLatency)||(sectionMap.route?.details?.chinaLatency)||[];
}

function latencyFill(ms,text){
  const t=String(text||'');
  if(/超时|timeout/i.test(t))return'rgba(255,107,120,0.62)';
  if(ms==null||ms<=0)return'rgba(141,168,178,0.16)';
  if(ms<=50)return'rgba(30,163,78,0.58)';
  if(ms<=100)return'rgba(71,223,140,0.58)';
  if(ms<=200)return'rgba(154,230,176,0.58)';
  if(ms<=250)return'rgba(255,200,87,0.58)';
  return'rgba(255,159,67,0.58)';
}

function provinceAvgLatencyByCode(list){
  const byCode=new Map();
  (list||[]).forEach(r=>{
    if(!r?.code)return;
    if(!byCode.has(r.code))byCode.set(r.code,{sum:0,count:0,text:''});
    const row=byCode.get(r.code);
    if(r.ms!=null&&r.ms>0){row.sum+=r.ms;row.count++;}
    if(/超时|timeout/i.test(r.text||''))row.text=r.text;
  });
  const out=new Map();
  byCode.forEach((row,code)=>{
    if(row.count>0)out.set(code,row.sum/row.count);
    else if(row.text)out.set(code,-1);
  });
  return out;
}

function renderChinaMapBg(){
  const el=$('#certificateMapBg');
  if(!el)return;
  const map=window.CHINA_MAP;
  const list=chinaLatencyList();
  const latencyByCode=provinceAvgLatencyByCode(list);
  const show=map?.provinces?.length&&latencyByCode.size;
  if(!show){el.innerHTML='';el.hidden=true;return;}
  const vb=map.viewBox||'0 0 800 600';
  const paths=map.provinces.map(p=>{
    const ms=latencyByCode.get(p.code);
    const fill=ms==null?'rgba(141,168,178,0.12)':ms<0?'rgba(255,107,120,0.62)':latencyFill(ms);
    return `<path class="map-prov" data-code="${esc(p.code)}" d="${p.path}" fill="${fill}" stroke="rgba(217,184,93,0.22)" stroke-width="0.7"/>`;
  }).join('');
  el.innerHTML=`<svg class="china-map-svg" viewBox="${vb}" preserveAspectRatio="xMidYMid meet" aria-hidden="true">${paths}</svg>`;
  el.hidden=false;
}

const CHINA_PROVINCE_ORDER=['BJ','TJ','HE','SX','NM','LN','JL','HL','SH','JS','ZJ','AH','FJ','JX','SD','HA','HB','HN','GD','GX','HI','CQ','SC','GZ','YN','XZ','SN','GS','QH','NX','XJ','HK','MO','TW'];
const CHINA_PROVINCE_META={
  BJ:{short:'京',name:'北京'},TJ:{short:'津',name:'天津'},HE:{short:'冀',name:'河北'},SX:{short:'晋',name:'山西'},
  NM:{short:'蒙',name:'内蒙古'},LN:{short:'辽',name:'辽宁'},JL:{short:'吉',name:'吉林'},HL:{short:'黑',name:'黑龙江'},
  SH:{short:'沪',name:'上海'},JS:{short:'苏',name:'江苏'},ZJ:{short:'浙',name:'浙江'},AH:{short:'皖',name:'安徽'},
  FJ:{short:'闽',name:'福建'},JX:{short:'赣',name:'江西'},SD:{short:'鲁',name:'山东'},HA:{short:'豫',name:'河南'},
  HB:{short:'鄂',name:'湖北'},HN:{short:'湘',name:'湖南'},GD:{short:'粤',name:'广东'},GX:{short:'桂',name:'广西'},
  HI:{short:'琼',name:'海南'},CQ:{short:'渝',name:'重庆'},SC:{short:'川',name:'四川'},GZ:{short:'贵',name:'贵州'},
  YN:{short:'云',name:'云南'},XZ:{short:'藏',name:'西藏'},SN:{short:'陕',name:'陕西'},GS:{short:'甘',name:'甘肃'},
  QH:{short:'青',name:'青海'},NX:{short:'宁',name:'宁夏'},XJ:{short:'新',name:'新疆'},
  HK:{short:'港',name:'香港'},MO:{short:'澳',name:'澳门'},TW:{short:'台',name:'台湾'}
};
function chinaProvinceOrderIndex(code){
  const i=CHINA_PROVINCE_ORDER.indexOf(code);
  return i>=0?i:999;
}
function chinaProvinceLabel(row){
  return row?.name||row?.short||row?.code||'—';
}
function chinaProvinceByCode(list){
  const byCode=new Map();
  CHINA_PROVINCE_ORDER.forEach(code=>{
    const meta=CHINA_PROVINCE_META[code]||{};
    byCode.set(code,{short:meta.short||code,name:meta.name||code,cells:{}});
  });
  (list||[]).forEach(r=>{
    if(!r?.code)return;
    if(!byCode.has(r.code)){
      byCode.set(r.code,{short:r.short,name:r.name,cells:{}});
    }
    const row=byCode.get(r.code);
    if(r.short)row.short=r.short;
    if(r.name)row.name=r.name;
    row.cells[r.carrier]=r;
  });
  return byCode;
}

function chinaProvinceMatrix(){
  const list=chinaLatencyList();
  if(!list.length)return'';
  const carriers=['电信','联通','移动'];
  const colsPerRow=2;
  const byCode=chinaProvinceByCode(list);
  const codeOrder=CHINA_PROVINCE_ORDER.slice();
  const groups=[];
  for(let i=0;i<codeOrder.length;i+=colsPerRow)groups.push(codeOrder.slice(i,i+colsPerRow));
  let table='<table class="ping-matrix province-matrix province-matrix-pair"><thead><tr>';
  for(let g=0;g<colsPerRow;g++)table+='<th></th>'+carriers.map(c=>`<th>${esc(c)}</th>`).join('');
  table+='</tr></thead><tbody>';
  groups.forEach(chunk=>{
    table+='<tr>';
    for(let g=0;g<colsPerRow;g++){
      const code=chunk[g];
      if(!code){table+='<th>—</th>'+carriers.map(()=>'<td>—</td>').join('');continue;}
      const row=byCode.get(code);
      const avgLoss=provinceAvgLoss(row.cells);
      table+=`<th><span class="prov-short">${esc(chinaProvinceLabel(row))}</span>${lossDot(avgLoss)}</th>`;
      table+=carriers.map(c=>{
        const hit=row.cells[c];
        const val=hit?.ms!=null&&hit.ms>0?coloredMs(hit.ms,hit.text):colorizeTextMs(hit?.text||'—');
        return `<td>${val}</td>`;
      }).join('');
    }
    table+='</tr>';
  });
  table+='</tbody></table>';
  return `<div class="province-matrix-wrap">${table}</div>`;
}

function intlPingGrid(){
  const rows=(sectionMap.network?.details?.intlLatency)||[];
  if(!rows.length)return'';
  return svcGridHtml(rows.map(r=>intlPingRow(r.city,r.ms,r.text)),6);
}

function routeReturnLineMatrix(){
  const routes=(sectionMap.route?.details?.returnRoutes)||[];
  if(!routes.length)return'';
  const cities=['北京','上海','广州'];
  const carriers=['电信','联通','移动'];
  const body=carriers.map(carrier=>{
    const cells=cities.map(city=>{
      const r=routes.find(x=>x.city===city&&x.carrier===carrier);
      const line=(r?.labels||[]).join('→')||r?.line||'—';
      return `<td>${routeLineHtml(line)}</td>`;
    }).join('');
    return `<tr><th>${esc(carrier)}</th>${cells}</tr>`;
  }).join('');
  return `<table class="route-return-matrix ping-matrix"><thead><tr><th>运营商</th>${cities.map(c=>`<th>${esc(c)}</th>`).join('')}</tr></thead><tbody>${body}</tbody></table>`;
}

function tcpPolicyRows(){
  const net=sectionMap.network||{};
  const p=(net.details||{}).tcpPolicy||{};
  const sys=sectionMap.system||{};
  const pick=(name,fallback='—')=>metricText(net.metrics,name,metricText(sys.metrics,name,fallback));
  const nat=pick('NAT',p.nat||'—');
  const natCell=/开放网络无NAT/.test(nat)?ipTag(nat,'ok'):/NAT/.test(nat)?ipTag(nat,'warn'):ipTag(nat,'muted');
  const bbr=pick('BBR',p.bbr||'—');
  const bbrCell=/已启用|enabled/i.test(bbr)?ipTag(bbr,'ok'):ipTag(bbr,'muted');
  return [
    ['NAT 类型',natCell,true],
    ['TCP 拥塞',pick('TCP 拥塞',p.congestion||'—'),true],
    ['队列调度',pick('队列调度',p.qdisc||'—'),true],
    ['BBR',bbrCell,true],
    ['TCP rmem',p.tcpRmem||pick('TCP 接收缓冲区上限','—'),true],
    ['TCP wmem',p.tcpWmem||pick('TCP 发送缓冲区上限','—'),true]
  ];
}

function speedMbpsCell(mbps,text){
  const label=text||(mbps>0?`${Number(mbps).toLocaleString(undefined,{maximumFractionDigits:1})} Mbps`:'—');
  if(mbps>0)return `<span class="speed-value">${esc(label)}</span>`;
  return ipTag(label,'bad');
}

function speedTestBlock(){
  const st=(sectionMap.network?.details||{}).speedTest||{};
  if(!st.downloadText&&!st.uploadText)return'';
  const rows=[
    ['下载',speedMbpsCell(st.downloadMbps,st.downloadText),true],
    ['上传',speedMbpsCell(st.uploadMbps,st.uploadText),true]
  ];
  return `<h3>带宽测速</h3>${kvTableHtml(rows)}`;
}

function udpStatusCell(label,ok){
  return ok?ipTag(label,'ok'):ipTag(label,'bad');
}

function udpProbeBlock(){
  const udp=(sectionMap.network?.details||{}).udpProbe||{};
  const net=sectionMap.network||{};
  const egress=udp.egressText||metricText(net.metrics,'UDP 出站','—');
  const nat=udp.natType||metricText(net.metrics,'UDP NAT','—');
  const quic=udp.quicText||metricText(net.metrics,'QUIC','—');
  const stun=maskSTUNMapping(udp);
  if(!egress&&!nat&&!quic&&stun==='—')return'';
  const rows=[
    ['UDP 出站',/不可用|失败|超时/.test(egress)?udpStatusCell(egress,false):coloredMs(udp.egressMs,egress),true],
    ['UDP NAT',/对称|限制|NAT/.test(nat)?ipTag(nat,'warn'):/公网|直连/.test(nat)?ipTag(nat,'ok'):ipTag(nat,'muted'),true],
    ['STUN 映射',stun==='—'?ipTag(stun,'muted'):esc(stun),true],
    ['QUIC',/不可用|失败/.test(quic)?udpStatusCell(quic,false):ipTag(quic,'ok'),true]
  ];
  let html=`<h3>UDP 检测</h3>${kvTableHtml(rows)}`;
  const targets=Array.isArray(udp.targets)?udp.targets:[];
  const targetRows=targets.filter(t=>t&&t.text).map(t=>[
    t.name||t.host||'节点',
    t.ms>0?coloredMs(t.ms,t.text):ipTag(t.text||'超时','bad'),
    true
  ]);
  if(targetRows.length){
    html+=`<h3>UDP 节点</h3>${kvTableHtml(targetRows)}`;
  }
  return html;
}

function networkColumn(){
  const net=sectionMap.network||{};
  const connectNames=new Set(['IPv4','IPv6','DNS','TCP 1.1.1.1']);
  const connectRows=(net.metrics||[]).filter(m=>connectNames.has(m.name)).map(m=>[m.name,metricCell(m),true]);
  const policy=tcpPolicyRows();
  const speedBlock=speedTestBlock();
  const udpBlock=udpProbeBlock();
  const chinaGrid=chinaPingGrid();
  const provinceGrid=chinaProvinceMatrix();
  const intlGrid=intlPingGrid();
  const hasLatencyLegend=!!(chinaGrid||provinceGrid||intlGrid);
  let html='<div class="col-panel">';
  if(policy.some(r=>r[1]&&r[1]!=='—'))html+=`<h3>本地策略</h3>${kvTableHtml(policy)}`;
  if(connectRows.length)html+=`<h3>基础连通</h3>${kvTableHtml(connectRows)}`;
  if(udpBlock)html+=udpBlock;
  if(speedBlock)html+=speedBlock;
  if(chinaGrid)html+=`<h3>三网延迟</h3>${chinaGrid}`;
  if(provinceGrid)html+=`<h3>全国延迟</h3>${provinceGrid}`;
  if(intlGrid)html+=`<h3>国际互连</h3>${intlGrid}`;
  if(hasLatencyLegend)html+=latencyLegendHtml();
  if(!connectRows.length&&!speedBlock&&!udpBlock&&!chinaGrid&&!provinceGrid&&!intlGrid)html+='<p class="route-note">无数据</p>';
  html+='</div>';
  return html;
}

function routeBriefSummaryTable(){
  const matrix=routeReturnLineMatrix();
  if(!matrix)return'';
  return `<h3>简评总结</h3>${matrix}`;
}

function routeColumn(){
  const routes=routeRows();
  const hopRoutes=routeHopRows();
  const briefTable=routeBriefSummaryTable();
  if(!routes&&!hopRoutes&&!briefTable)return `<div class="col-panel"><p class="route-note">无回程数据</p></div>`;
  let html='<div class="col-panel">';
  if(briefTable)html+=briefTable;
  if(routes)html+=`<h3>节点详情</h3><table class="route-table"><colgroup><col class="route-col-node"><col class="route-col-line"><col class="route-col-ping"></colgroup><thead><tr><th>节点</th><th>路线图</th><th>延迟</th></tr></thead><tbody>${routes}</tbody></table>`;
  if(hopRoutes)html+=`<h3>IP 路径</h3><table class="route-table route-table-ip"><colgroup><col class="route-col-node"><col class="route-col-line"></colgroup><thead><tr><th>节点</th><th>IP 路径</th></tr></thead><tbody>${hopRoutes}</tbody></table>`;
  html+='</div>';
  return html;
}

function colSection(id,title,bodyFn){
  return `<section class="report-col" id="report-col-${id}"><header class="col-head"><h2>${title}</h2></header><div class="col-body">${bodyFn()}</div></section>`;
}
function renderCertificate(){
  if(!Object.keys(sectionMap).length)return;
  empty.hidden=true;
  document.body.classList.add('fit-active');
  sections.className='report-body';
  sections.innerHTML=`<div class="report-columns">${colSection('system','系统报告',systemColumn)}${colSection('ip','IP 质量',ipColumn)}${colSection('network','网络质量',networkColumn)}${colSection('route','回程路由',routeColumn)}</div>`;
  renderChinaMapBg();
  scheduleFit();
}

function ingestSection(s){
  sectionMap[s.id]=s;
  renderCertificate();
}

function scheduleFit(){requestAnimationFrame(()=>requestAnimationFrame(fitCertificate))}

function fitCertificate(){
  const canvas=$('#reportCanvas'),fit=$('#certificateFit'),cert=fit?.querySelector('.certificate'),scaleWrap=$('#certBodyScale'),inner=scaleWrap?.querySelector('.cert-body-inner');
  if(!canvas||!fit||!cert||!scaleWrap||!inner)return;
  const hasReport=empty?.hidden;
  if(!hasReport){
    document.body.classList.remove('fit-active');
    fit.style.cssText='';cert.style.cssText='';scaleWrap.style.cssText='';inner.style.cssText='';
    return;
  }
  cert.style.width=DESIGN_W+'px';cert.style.height=DESIGN_H+'px';
  scaleWrap.style.cssText='';inner.style.cssText='width:100%;height:100%;display:flex;flex-direction:column';
  cert.style.transform='none';

  const bodyH=scaleWrap.clientHeight||0;
  inner.style.transform='none';inner.style.width='100%';inner.style.height='auto';
  const innerH=inner.scrollHeight||0;
  const innerW=inner.scrollWidth||DESIGN_W;
  let bodyScale=1;
  if(bodyH>0&&innerH>bodyH)bodyScale=Math.min(bodyScale,bodyH/innerH);
  if(innerW>DESIGN_W-32)bodyScale=Math.min(bodyScale,(DESIGN_W-32)/innerW);
  document.querySelectorAll('.col-body').forEach(col=>{
    if(col.scrollHeight>col.clientHeight&&col.clientHeight>0){
      bodyScale=Math.min(bodyScale,col.clientHeight/col.scrollHeight);
    }
  });
  if(bodyScale<1){
    inner.style.transform=`scale(${bodyScale})`;
    inner.style.transformOrigin='top left';
    inner.style.width=(100/bodyScale)+'%';
    scaleWrap.style.height=Math.ceil(innerH*bodyScale)+'px';
  }else{
    scaleWrap.style.height='100%';
  }

  const canvasW=canvas.clientWidth,canvasH=canvas.clientHeight;
  if(!canvasW||!canvasH)return;
  const scale=Math.min(canvasW/DESIGN_W,canvasH/DESIGN_H);
  const scaledW=DESIGN_W*scale,scaledH=DESIGN_H*scale;
  fit.style.left=((canvasW-scaledW)/2)+'px';
  fit.style.top=((canvasH-scaledH)/2)+'px';
  fit.style.width=scaledW+'px';
  fit.style.height=scaledH+'px';
  cert.style.transform=`scale(${scale})`;
  cert.style.transformOrigin='top left';
}

function bindFitObserver(){
  if(fitObserver)return;
  fitObserver=new ResizeObserver(()=>scheduleFit());
  const canvas=$('#reportCanvas');
  if(canvas)fitObserver.observe(canvas);
  window.addEventListener('resize',scheduleFit,{passive:true});
  window.addEventListener('orientationchange',()=>setTimeout(scheduleFit,120),{passive:true});
}

function busy(v){running=v;$('#run').disabled=v}

async function run(){
  if(running)return;
  sectionMap={};empty.hidden=false;sections.className='report-body';sections.innerHTML='';document.body.classList.remove('fit-active');
  busy(true);
  const r=await fetch('/api/run',{method:'POST'});
  if(!r.ok){busy(false)}
}

async function renderReport(r){
  sectionMap={};
  (r.sections||[]).forEach(s=>{sectionMap[s.id]=s});
  $('#score').textContent=r.score||'—';
  $('#certificateId').textContent=(r.id||'—').toUpperCase();
  $('#certificateHost').textContent=maskHostname(r.hostname);
  $('#certificatePlatform').textContent=r.platform||'—';
  $('#certificateTime').textContent=r.startedAt?new Date(r.startedAt).toLocaleString():'—';
  renderCertificate();
}

async function loadLatest(){
  const id=location.pathname.startsWith('/report/')?location.pathname.split('/').pop():'';
  const url=id?`/api/reports/${encodeURIComponent(id)}`:'/api/reports/latest';
  const r=await fetch(url);
  if(!r.ok){scheduleFit();return;}
  const data=await r.json();
  if(data&&Array.isArray(data.sections)&&data.sections.length)renderReport(data);
  else scheduleFit();
}

$('#run').onclick=()=>run();
$('#theme').onclick=()=>{const d=document.documentElement;d.dataset.theme=d.dataset.theme==='light'?'dark':'light';localStorage.theme=d.dataset.theme};
document.documentElement.dataset.theme=localStorage.theme||'dark';

let eventSource, eventRetryMs=2000;
function connectEvents(){
  if(eventSource)eventSource.close();
  eventSource=new EventSource('/api/events');
  eventSource.addEventListener('ready',()=>{eventRetryMs=2000});
  eventSource.addEventListener('started',()=>{busy(true);sectionMap={};empty.hidden=false;sections.className='report-body';sections.innerHTML=''});
  eventSource.addEventListener('section',e=>ingestSection(JSON.parse(e.data).section));
  eventSource.addEventListener('completed',async()=>{busy(false);await loadLatest()});
  eventSource.onerror=()=>{
    eventSource.close();
    setTimeout(connectEvents,eventRetryMs);
    eventRetryMs=Math.min(eventRetryMs*2,15000);
  };
}

bindFitObserver();connectEvents();loadLatest();
