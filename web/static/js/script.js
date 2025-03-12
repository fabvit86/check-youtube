function jsScript() {
    const serverBasepath = document.querySelector('meta[name="server-basepath"]')
        .getAttribute('content');

    // handle filters events
    handleFilters()

    // convert timestamps to locale
    convertTimestampsToLocale()

    // hadle mark as viewed buttons event
    markAsViewed(serverBasepath)
    markAllAsViewed(serverBasepath)

    // sort table by column on click
    sortByColumn(serverBasepath)
}

// call the backend endpoint and remove table row when user clicks on "mark as viewed"
function markAsViewed(serverBasepath) {
    const markAsViewedButtons = document.querySelectorAll('button.mark-as-viewed')
    markAsViewedButtons.forEach((btn) => {
        btn.addEventListener('click', async function(event) {
            const channelID = btn.dataset.channelid;
            const trID = event.target.closest("tr").id;
            try {
                let totVids = Number(document.getElementById("tot-channels").textContent);

                // Frontend-based workaround
                await visitChannel(channelID);

                // await fetch(serverBasepath + "/mark-as-viewed", {
                //     method: 'POST',
                //     body: JSON.stringify({channels_id: [channelID]})
                // });

                document.getElementById(trID).remove();
                document.getElementById("tot-channels").textContent = totVids - 1;
            } catch (e) {
                console.log(e);
                return e;
            }
        });
    });
}
function markAllAsViewed(serverBasepath) {
    // collect all channels IDs
    const tableContentRows = document.querySelectorAll('table#videos-table tbody tr')
    let channelsID = [];
    tableContentRows.forEach((tr) => {
        channelsID.push(tr.dataset.channelid);
    });

    if (channelsID.length === 0) {
        return;
    }

    const markAllAsViewedButton = document.querySelector('button#mark-all-as-viewed')
    markAllAsViewedButton.addEventListener('click', async function() {
        try {
            // Frontend-based workaround
            for (const channelID of channelsID) {
                await visitChannel(channelID);
            }

            // await fetch(serverBasepath + "/mark-as-viewed", {
            //     method: 'POST',
            //     body: JSON.stringify({channels_id: channelsID})
            // });

            // clear table body
            document.querySelector('table#videos-table tbody').innerHTML = "";
            document.getElementById("tot-channels").textContent = "0";
        } catch (e) {
            console.log(e);
            return e;
        }
    });
}

// open the link in a new tab and immediately close the tab (works only for the YT account logged-in on the browser)
function visitChannel(channelID) {
    const url = "https://www.youtube.com/channel/" + channelID;
    window.open(url, "_blank").close();
    return new Promise(resolve => setTimeout(resolve, 1));
}

// handle results filters
function handleFilters() {
    // reload page when user clicks on "show all" or "show filtered"
    document.getElementById("filters-div").addEventListener("click", function(e) {
        if(e.target.id === "show-all-btn") {
            window.open(window.location.href.split('?')[0],'_self');
        } else {
            window.open('?filtered=true','_self');
        }
    });

    // highlight the proper filters button and update info text
    let params = new URLSearchParams(document.location.search);
    let filtered = params.get("filtered");
    if (filtered === "true") {
        document.getElementById("show-filtered-btn").classList.add("active");
        document.getElementById("show-all-btn").classList.remove("active");
        document.getElementById("channels-info-span").textContent="# of channels with new videos:";
    } else {
        document.getElementById("show-all-btn").classList.add("active");
        document.getElementById("show-filtered-btn").classList.remove("active");
        document.getElementById("channels-info-span").textContent="# of channels:";
        document.querySelectorAll(".mark-as-viewed").forEach(el => el.remove());
        document.getElementById("mark-all-as-viewed").remove();
    }
}

// convert timestamps to locale
function convertTimestampsToLocale() {
    const timestampElements = document.querySelectorAll('span[data-ts]');
    timestampElements.forEach((element) => {
        element.innerText = new Date(element.dataset.ts).toLocaleString();
    });
}

// sort results by column on click
function sortByColumn(serverBasepath) {
    const downArrow = "↓";
    const upArrow = "↑";
    const doubleArrow = "⇅";
    const separator  = '-';
    const table = document.getElementById("videos-table");

    table.querySelectorAll('th.sortable').forEach((th) => {
        th.addEventListener('click', function() {
            let values = [];
            let rowsMap = {}; // key = channel name, value = html of the row
            let sortArrow = this.querySelector('span.sort-arrow');

            // collect values to sort from each table row
            table.querySelectorAll('tbody tr').forEach((row, index) => {
                const cell = row.children[th.cellIndex]
                const content = cell.querySelector('span[data-ts]') != null
                    ? cell.querySelector('span[data-ts]').dataset.ts
                    : cell.textContent.toLowerCase();
                values.push(content + separator + index);
                rowsMap[content + separator + index] = row.outerHTML;
            });

            // sort elements
            if (sortArrow.innerHTML === upArrow) {
                // descending order
                values.sort((a, b) => b.localeCompare(a));
                sortArrow.innerHTML = downArrow;
            } else {
                // ascending order
                values.sort();
                sortArrow.innerHTML = upArrow;
            }

            // change the sort arrow of other th elements to double arrow
            table.querySelectorAll(`th.sortable:not(#${th.id})`).forEach((th) => {
                th.querySelector('span.sort-arrow').innerHTML = doubleArrow;
            });

            // update table body
            let tableBody = '';
            values.forEach(key => tableBody += rowsMap[key]);
            table.getElementsByTagName('tbody')[0].innerHTML = tableBody;

            // reapply markAsViewed event listener to table elements
            markAsViewed(serverBasepath);
        });
    });
}
